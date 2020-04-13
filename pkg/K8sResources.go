package pkg

import (
	"encoding/pem"
	"fmt"
	"github.com/fatih/color"
	"github.com/funceasy/funceasy-cli/pkg/util"
	"github.com/funceasy/funceasy-cli/pkg/util/terminal"
	"github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"time"
)

const NAMESPACE string = "funceasy"

func NewK8sClientSet() (kubernetesClient *kubernetes.Clientset, apiExtensionsClient *apiextensionsclient.Clientset) {
	var kubeConfig string
	if home := homedir.HomeDir(); home != "" {
		kubeConfig = filepath.Join(home, ".kube", "config")
	} else {
		kubeConfig = ""
	}
	cfg, err := clientcmd.BuildConfigFromFlags("", kubeConfig)
	if err != nil {
		logrus.Fatal(err)
	}
	clientSet, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		logrus.Fatal(err)
	}
	apiExtensionsClientSet, err := apiextensionsclient.NewForConfig(cfg)
	if err != nil {
		logrus.Fatal(err)
	}
	return clientSet, apiExtensionsClientSet
}

func DeployFuncEasyResources(fileByte []byte, PVType string, pathOrClass string) error {
	err := v1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	t := terminal.NewTerminalPrint()
	objectList, err := util.ParseK8sYaml(fileByte)
	if err != nil {
		return err
	}
	clientSet, apiExtensionsClientSet := NewK8sClientSet()

	configMapClient := clientSet.CoreV1().ConfigMaps(NAMESPACE)
	deploymentClient := clientSet.AppsV1().Deployments(NAMESPACE)
	serviceClient := clientSet.CoreV1().Services(NAMESPACE)
	secretClient := clientSet.CoreV1().Secrets(NAMESPACE)
	PVClient := clientSet.CoreV1().PersistentVolumes()
	PVCClient := clientSet.CoreV1().PersistentVolumeClaims(NAMESPACE)
	SAClient := clientSet.CoreV1().ServiceAccounts(NAMESPACE)
	RoleClient := clientSet.RbacV1().Roles(NAMESPACE)
	RBClient := clientSet.RbacV1().RoleBindings(NAMESPACE)
	CRDClient := apiExtensionsClientSet.ApiextensionsV1beta1().CustomResourceDefinitions()
	var rollback []func() error
	defer func() {
		if err := recover(); err != nil {
			util.Rollback(rollback, t)
		}
	}()
	var nodePort = make(map[string]int32)
	for _, item := range objectList {
		switch item.(type) {
		case *coreV1.ConfigMap:
			configMap := item.(*coreV1.ConfigMap)
			t.PrintInfoOneLine("Creating ConfigMap: %s", configMap.Name)
			_, err := configMapClient.Create(configMap)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("ConfigMap: %s Created", configMap.Name)
			t.LineEnd()
			cb := configMapClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, configMap.Name, &metaV1.DeleteOptions{}))
		case *appsV1.Deployment:
			deployment := item.(*appsV1.Deployment)
			t.PrintInfoOneLine("Creating Deployment: %s", deployment.Name)
			_, err := deploymentClient.Create(deployment)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("Deployment: %s Created", deployment.Name)
			t.LineEnd()
			cb := deploymentClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, deployment.Name, &metaV1.DeleteOptions{}))
			if deployment.Name == "funceasy-mysql"  {
				result := make(chan string)
				done := make(chan bool)
				spinningDone := make(chan bool)
				appLabel := deployment.Spec.Template.Labels["app"]
				do := CheckMysqlPodsRunning(clientSet, appLabel)
				t.PrintLoadingOneLine(spinningDone, "Waiting %s Start", deployment.Name)
				util.PollingCheck(do, 5 * time.Second ,result, done)
				resultStr := <-result
				<-time.After(30 * time.Second)
				spinningDone<-true
				t.PrintSuccessOneLine("%s Started", deployment.Name)
				if resultStr == "failed" {
					t.PrintErrorOneLineWithPanic(err)
				}
			}
		case *coreV1.Service:
			service := item.(*coreV1.Service)
			t.PrintInfoOneLine("Creating Service: %s", service.Name)
			_, err := serviceClient.Create(service)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("Service: %s Created", service.Name)
			t.LineEnd()
			if service.Spec.Type == coreV1.ServiceTypeNodePort {
				nodePort[service.Name] = service.Spec.Ports[0].NodePort
			}
			cb := serviceClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, service.Name, &metaV1.DeleteOptions{}))
		case *coreV1.Secret:
			secret := item.(*coreV1.Secret)
			t.PrintInfoOneLine("Creating Secret: %s", secret.Name)
			if secret.Labels["generatedBy"] == "cli" {
				keyName := secret.Labels["keyName"]
				privateKeyPemBlock, publicKeyPemBlock, err := GenerateRSAKeys(1024)
				if err != nil {
					t.PrintErrorOneLineWithPanic(err)
				}
				publicKeyByte := pem.EncodeToMemory(publicKeyPemBlock)
				privateByte := pem.EncodeToMemory(privateKeyPemBlock)
				tokenStr, err := SignedToken(keyName, privateByte)
				if err != nil {
					t.PrintErrorOneLineWithPanic(err)
				}
				secret.Data = map[string][]byte{
					keyName + ".public.key": publicKeyByte,
					keyName + ".token": []byte(tokenStr),
				}
			}
			_, err := secretClient.Create(secret)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("Secret: %s Created", secret.Name)
			t.LineEnd()
			cb := secretClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, secret.Name, &metaV1.DeleteOptions{}))
		case *coreV1.PersistentVolumeClaim:
			pvc := item.(*coreV1.PersistentVolumeClaim)
			if PVType == "Local" {
				dirName := "funceasy-" + pvc.Name + "-volume"
				dirPath := path.Join(pathOrClass, dirName)
				if _, err := os.Stat(dirPath); os.IsNotExist(err) {
					err = os.Mkdir(dirPath, os.ModePerm)
					if err != nil {
						t.PrintErrorOneLineWithPanic(err)
					}
					err = os.Chmod(dirPath, os.ModePerm)
					if err != nil {
						t.PrintErrorOneLineWithPanic(err)
					}
				}
				pv := &coreV1.PersistentVolume{
					ObjectMeta: metaV1.ObjectMeta{
						Name: dirName,
					},
					Spec:       coreV1.PersistentVolumeSpec{
						PersistentVolumeReclaimPolicy: coreV1.PersistentVolumeReclaimRecycle,
						Capacity: pvc.Spec.Resources.Requests,
						AccessModes: []coreV1.PersistentVolumeAccessMode{
							coreV1.ReadWriteOnce,
						},
						PersistentVolumeSource: coreV1.PersistentVolumeSource{
							HostPath:             &coreV1.HostPathVolumeSource{
								Path: dirPath,
							},
						},
					},
				}
				t.PrintInfoOneLine("Creating PV: %s", pv.Name)
				_, err := PVClient.Create(pv)
				if err != nil {
					if !errors.IsAlreadyExists(err) {
						t.PrintErrorOneLineWithPanic(err)
					}
					t.PrintWarnOneLine("AlreadyExists PV: %s", pv.Name)
					t.LineEnd()
				} else {
					t.PrintSuccessOneLine("PV: %s Created", pv.Name)
					t.LineEnd()
				}
				cb := PVClient.Delete
				rollback = append(rollback, util.GenerateDeleteCallback(cb, pv.Name, &metaV1.DeleteOptions{}))
			}
			if PVType == "StorageClass" {
				 pvc.Spec.StorageClassName = &pathOrClass
			}
			t.PrintInfoOneLine("Creating PVC: %s", pvc.Name)
			_, err := PVCClient.Create(pvc)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("PVC: %s Created", pvc.Name)
			t.LineEnd()
			cb := PVCClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, pvc.Name, &metaV1.DeleteOptions{}))
		case *coreV1.ServiceAccount:
			sa := item.(*coreV1.ServiceAccount)
			t.PrintInfoOneLine("Creating ServiceAccount: %s", sa.Name)
			_, err := SAClient.Create(sa)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("ServiceAccount: %s Created", sa.Name)
			t.LineEnd()
			cb := SAClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, sa.Name, &metaV1.DeleteOptions{}))
		case *rbacV1.Role:
			role := item.(*rbacV1.Role)
			t.PrintInfoOneLine("Creating Role: %s", role.Name)
			_, err := RoleClient.Create(role)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("Role: %s Created", role.Name)
			t.LineEnd()
			cb := RoleClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, role.Name, &metaV1.DeleteOptions{}))
		case *rbacV1.RoleBinding:
			rb := item.(*rbacV1.RoleBinding)
			t.PrintInfoOneLine("Creating RoleBinding: %s", rb.Name)
			_, err := RBClient.Create(rb)
			if err != nil {
				t.PrintErrorOneLineWithPanic(err)
			}
			t.PrintSuccessOneLine("RoleBinding: %s Created", rb.Name)
			t.LineEnd()
			cb := RBClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, rb.Name, &metaV1.DeleteOptions{}))
		case *v1beta1.CustomResourceDefinition:
			crd := item.(*v1beta1.CustomResourceDefinition)
			t.PrintInfoOneLine("Creating CRD: %s", crd.Name)
			_, err := CRDClient.Create(crd)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					t.PrintErrorOneLineWithPanic(err)
				}
				t.PrintWarnOneLine("AlreadyExists CRD: %s", crd.Name)
				t.LineEnd()
			} else {
				t.PrintSuccessOneLine("CRD: %s Created", crd.Name)
				t.LineEnd()
			}
			cb := CRDClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, crd.Name, &metaV1.DeleteOptions{}))
		}
	}
	for key, value := range nodePort {
		t.PrintInfoOneLine("Service [%s] exposed on NodePort -> %d", key, value)
		t.LineEnd()
	}
	return nil
}

func UpdateFuncEasyResources(fileByte []byte) error {
	err := v1beta1.AddToScheme(scheme.Scheme)
	if err != nil {
		return err
	}
	t := terminal.NewTerminalPrint()
	objectList, err := util.ParseK8sYaml(fileByte)
	if err != nil {
		return err
	}
	clientSet, apiExtensionsClientSet := NewK8sClientSet()

	configMapClient := clientSet.CoreV1().ConfigMaps(NAMESPACE)
	deploymentClient := clientSet.AppsV1().Deployments(NAMESPACE)
	secretClient := clientSet.CoreV1().Secrets(NAMESPACE)
	serviceClient := clientSet.CoreV1().Services(NAMESPACE)
	SAClient := clientSet.CoreV1().ServiceAccounts(NAMESPACE)
	RoleClient := clientSet.RbacV1().Roles(NAMESPACE)
	RBClient := clientSet.RbacV1().RoleBindings(NAMESPACE)
	CRDClient := apiExtensionsClientSet.ApiextensionsV1beta1().CustomResourceDefinitions()
	var nodePort = make(map[string]int32)
	for _, item := range objectList {
		switch item.(type) {
		case *coreV1.ConfigMap:
			configMapNew := item.(*coreV1.ConfigMap)
			t.PrintInfoOneLine("Updating ConfigMap: %s", configMapNew.Name)
			configMapOld, err := configMapClient.Get(configMapNew.Name, metaV1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					t.PrintWarnOneLine("ConfigMap Not Found and Creating: %s", configMapNew.Name)
					_, err := configMapClient.Create(configMapNew)
					if err != nil {
						t.PrintErrorOneLineWithExit(err)
					}
					t.PrintWarnOneLine("ConfigMap Not Found and Created: %s", configMapNew.Name)
					t.LineEnd()
				} else {
					t.PrintErrorOneLineWithExit(err)
				}
			} else {
				configMapOld.Data = configMapNew.Data
				_, err = configMapClient.Update(configMapOld)
				if err != nil {
					t.PrintErrorOneLineWithExit(err)
				}
				t.PrintSuccessOneLine("ConfigMap: %s Updated", configMapNew.Name)
				t.LineEnd()
			}
		case *appsV1.Deployment:
			deploymentNew := item.(*appsV1.Deployment)
			t.PrintInfoOneLine("Updating Deployment: %s", deploymentNew.Name)
			deploymentOld, err := deploymentClient.Get(deploymentNew.Name, metaV1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					t.PrintWarnOneLine("Deployment Not Found and Creating: %s", deploymentNew.Name)
					_, err := deploymentClient.Create(deploymentNew)
					if err != nil {
						t.PrintErrorOneLineWithExit(err)
					}
					t.PrintWarnOneLine("Deployment Not Found and Created: %s", deploymentNew.Name)
					t.LineEnd()
				} else {
					t.PrintErrorOneLineWithExit(err)
				}
			} else {
				deploymentOld.Spec = deploymentNew.Spec
				_, err = deploymentClient.Update(deploymentOld)
				if err != nil {
					t.PrintErrorOneLineWithExit(err)
				}
				t.PrintSuccessOneLine("Deployment: %s Updated", deploymentNew.Name)
				t.LineEnd()
			}
			if deploymentNew.Name == "funceasy-mysql" && !reflect.DeepEqual(deploymentNew.Spec, deploymentNew.Spec)  {
				result := make(chan string)
				done := make(chan bool)
				spinningDone := make(chan bool)
				appLabel := deploymentNew.Spec.Template.Labels["app"]
				do := CheckMysqlPodsRunning(clientSet, appLabel)
				util.PollingCheck(do, 5 * time.Second ,result, done)
				t.PrintLoadingOneLine(spinningDone, "Waiting %s Start", deploymentNew.Name)
				resultStr := <-result
				<-time.After(30 * time.Second)
				spinningDone<-true
				t.PrintSuccessOneLine("%s Started", deploymentNew.Name)
				if resultStr == "failed" {
					t.PrintErrorOneLineWithExit(err)
				}
			}
		case *coreV1.Secret:
			secret := item.(*coreV1.Secret)
			t.PrintInfoOneLine("Updating Secret: %s", secret.Name)
			_, err := secretClient.Get(secret.Name, metaV1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					t.PrintWarnOneLine("Secret Not Found and Creating: %s", secret.Name)
					if secret.Labels["generatedBy"] == "cli" {
						keyName := secret.Labels["keyName"]
						privateKeyPemBlock, publicKeyPemBlock, err := GenerateRSAKeys(1024)
						if err != nil {
							t.PrintErrorOneLineWithExit(err)
						}
						publicKeyByte := pem.EncodeToMemory(publicKeyPemBlock)
						privateByte := pem.EncodeToMemory(privateKeyPemBlock)
						tokenStr, err := SignedToken(keyName, privateByte)
						if err != nil {
							t.PrintErrorOneLineWithExit(err)
						}
						secret.Data = map[string][]byte{
							keyName + ".public.key": publicKeyByte,
							keyName + ".token": []byte(tokenStr),
						}
					}
					_, err := secretClient.Create(secret)
					if err != nil {
						t.PrintErrorOneLineWithExit(err)
					}
					t.PrintWarnOneLine("Deployment Not Found and Created: %s", secret.Name)
					t.LineEnd()
				} else {
					t.PrintErrorOneLineWithExit(err)
				}
			}
		case *coreV1.Service:
			service := item.(*coreV1.Service)
			t.PrintInfoOneLine("Updating Service: %s", service.Name)
			err := serviceClient.Delete(service.Name, &metaV1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.PrintErrorOneLineWithExit(err)
			}
			_, err = serviceClient.Create(service)
			if err != nil {
				t.PrintErrorOneLineWithExit(err)
			}
			t.PrintSuccessOneLine("Service: %s Updated", service.Name)
			t.LineEnd()
			if service.Spec.Type == coreV1.ServiceTypeNodePort {
				nodePort[service.Name] = service.Spec.Ports[0].NodePort
			}
		case *coreV1.ServiceAccount:
			sa := item.(*coreV1.ServiceAccount)
			t.PrintInfoOneLine("Updating ServiceAccount: %s", sa.Name)
			err := SAClient.Delete(sa.Name, &metaV1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.PrintErrorOneLineWithExit(err)
			}
			_, err = SAClient.Create(sa)
			if err != nil {
				t.PrintErrorOneLineWithExit(err)
			}
			t.PrintSuccessOneLine("ServiceAccount: %s Updated", sa.Name)
			t.LineEnd()
		case *rbacV1.Role:
			role := item.(*rbacV1.Role)
			t.PrintInfoOneLine("Updating Role: %s", role.Name)
			err := RoleClient.Delete(role.Name, &metaV1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.PrintErrorOneLineWithExit(err)
			}
			_, err = RoleClient.Create(role)
			if err != nil {
				t.PrintErrorOneLineWithExit(err)
			}
			t.PrintSuccessOneLine("Role: %s Updated", role.Name)
			t.LineEnd()
		case *rbacV1.RoleBinding:
			rb := item.(*rbacV1.RoleBinding)
			t.PrintInfoOneLine("Updating RoleBinding: %s", rb.Name)
			err := RBClient.Delete(rb.Name, &metaV1.DeleteOptions{})
			if err != nil && !errors.IsNotFound(err) {
				t.PrintErrorOneLineWithExit(err)
			}
			_, err = RBClient.Create(rb)
			if err != nil {
				t.PrintErrorOneLineWithExit(err)
			}
			t.PrintSuccessOneLine("RoleBinding: %s Updated", rb.Name)
			t.LineEnd()
		case *v1beta1.CustomResourceDefinition:
			crd := item.(*v1beta1.CustomResourceDefinition)
			t.PrintInfoOneLine("Updating CRD: %s", crd.Name)
			_, err := CRDClient.Get(crd.Name, metaV1.GetOptions{})
			if err != nil {
				if errors.IsNotFound(err) {
					_, err = CRDClient.Create(crd)
					if err != nil {
						t.PrintErrorOneLineWithExit(err)
					}
					t.PrintSuccessOneLine("CRD: %s Updated", crd.Name)
					t.LineEnd()
				} else {
					t.PrintErrorOneLineWithExit(err)
				}
			}
		}
	}
	for key, value := range nodePort {
		t.PrintInfoOneLine("Service [%s] exposed on NodePort -> %d", key, value)
		t.LineEnd()
	}
	return nil
}

func GetResourceStatus() {
	t := terminal.NewTerminalPrint()
	appList := []string{
		"function-operator",
		"funceasy-mysql",
		"data-source-service",
		"funceasy-gateway",
		"funceasy-api",
		"funceasy-website",
	}
	status := make(map[string][]coreV1.PodPhase)
	clientSet, _ := NewK8sClientSet()
	for _, item := range appList  {
		podLabels := metaV1.LabelSelector{
			MatchLabels: map[string]string{
				"app": item,
			},
		}
		pods, err := clientSet.CoreV1().Pods(NAMESPACE).List(metaV1.ListOptions{
			LabelSelector:       labels.Set(podLabels.MatchLabels).String(),
		})
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		var podsStatus []coreV1.PodPhase
		for _, pod := range pods.Items {
			podsStatus = append(podsStatus, pod.Status.Phase)
		}
		status[item] = podsStatus
	}
	for key, value := range status  {
		t.PrintInfoOneLine("Pods: %s", key)
		t.LineEnd()
		str := ""
		if len(value) == 0 {
			str = color.YellowString("No Pods Running")
		} else {
			for index, item := range value {
				textWithColor := ""
				if item == coreV1.PodRunning {
					textWithColor = color.HiGreenString(string(item))
				} else if item == coreV1.PodFailed {
					textWithColor = color.HiRedString(string(item))
				} else {
					textWithColor = string(item)
				}
				str = str + fmt.Sprintf("[%d] %s\t", index, textWithColor)
			}
		}
		fmt.Println(str)
	}
}

func Restart() {
	t := terminal.NewTerminalPrint()
	appList := []string{
		"data-source-service",
		"funceasy-gateway",
		"funceasy-api",
		"funceasy-website",
	}
	clientSet, _ := NewK8sClientSet()
	for _, item := range appList  {
		t.PrintWarnOneLine("Restarting %s", item)
		podLabels := metaV1.LabelSelector{
			MatchLabels: map[string]string{
				"app": item,
			},
		}
		pods, err := clientSet.CoreV1().Pods(NAMESPACE).List(metaV1.ListOptions{
			LabelSelector:       labels.Set(podLabels.MatchLabels).String(),
		})
		if err != nil {
			t.PrintErrorOneLineWithExit(err)
		}
		for _, pod := range pods.Items {
			err := clientSet.CoreV1().Pods(NAMESPACE).Delete(pod.Name, &metaV1.DeleteOptions{})
			if err != nil {
				t.PrintErrorOneLineWithExit(err)
			}
		}
		t.PrintSuccessOneLine("Restarted %s", item)
		t.LineEnd()
	}
}

func GetCurrentVersion() string {
	clientSet, _ := NewK8sClientSet()
	t := terminal.NewTerminalPrint()
	cm, err := clientSet.CoreV1().ConfigMaps("funceasy").Get("funceasy-config", metaV1.GetOptions{})
	if err != nil {
		if !errors.IsNotFound(err) {
			t.PrintErrorOneLine("Get Current Version Failed \n", err)
		} else {
			return ""
		}
	}
	return cm.Data["version"]
}

func CheckMysqlPodsRunning(clientSet *kubernetes.Clientset, appLabel string) func(result chan string, done chan bool) error {
	podLabels := metaV1.LabelSelector{
		MatchLabels: map[string]string{
			"app": appLabel,
		},
	}
	return func(result chan string, done chan bool) error {
		pods, err := clientSet.CoreV1().Pods(NAMESPACE).List(metaV1.ListOptions{
			LabelSelector:      labels.Set(podLabels.MatchLabels).String(),
		})
		if err != nil {
			return err
		}
		if len(pods.Items) > 0 {
			pod := pods.Items[0]
			if pod.Status.Phase == coreV1.PodRunning {
				for _, item := range pod.Status.Conditions {
					if item.Type == coreV1.PodReady && item.Status == coreV1.ConditionTrue {
						done<-true
						result<-"success"
					}
				}
			}
			if pod.Status.Phase == coreV1.PodFailed {
				done<-true
				result<-"failed"
			}
		} else {
			return nil
		}
		return nil
	}
}
