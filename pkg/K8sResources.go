package pkg

import (
	"encoding/pem"
	"github.com/funceasy/funceasy-cli/pkg/util"
	"github.com/sirupsen/logrus"
	appsV1 "k8s.io/api/apps/v1"
	coreV1 "k8s.io/api/core/v1"
	rbacV1 "k8s.io/api/rbac/v1"
	apiextensionsclient "k8s.io/apiextensions-apiserver/pkg/client/clientset/clientset"
	"k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1beta1"
	"k8s.io/apimachinery/pkg/api/errors"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/util/homedir"
	"path/filepath"
	"time"
)

const NAMESPACE string = "funceasy"

func NewK8sClientSet() (*kubernetes.Clientset, *apiextensionsclient.Clientset) {
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
			util.Rollback(rollback)
		}
	}()
	for _, item := range objectList {
		switch item.(type) {
		case *coreV1.ConfigMap:
			configMap := item.(*coreV1.ConfigMap)
			logrus.Info("Creating ConfigMap: ", configMap.Name)
			_, err := configMapClient.Create(configMap)
			if err != nil {
				logrus.Panic(err)
			}
			cb := configMapClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, configMap.Name, &metaV1.DeleteOptions{}))
		case *appsV1.Deployment:
			deployment := item.(*appsV1.Deployment)
			logrus.Info("Creating Deployment: ", deployment.Name)
			_, err := deploymentClient.Create(deployment)
			if err != nil {
				logrus.Panic(err)
			}
			cb := deploymentClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, deployment.Name, &metaV1.DeleteOptions{}))
			if deployment.Name == "funceasy-mysql"  {
				result := make(chan string)
				done := make(chan bool)
				appLabel := deployment.Spec.Template.Labels["app"]
				do := CheckMysqlPodsRunning(clientSet, appLabel)
				util.PollingCheck(do, 5 * time.Second ,result, done)
				logrus.Info("waiting")
				resultStr := <-result
				logrus.Info("waiting 30 seconds for start")
				<-time.After(30 * time.Second)
				logrus.Info("waiting done")
				if resultStr == "failed" {
					logrus.Panic("Start Mysql Pod Failed")
				}
			}
		case *coreV1.Service:
			service := item.(*coreV1.Service)
			logrus.Info("Creating Service: ", service.Name)
			_, err := serviceClient.Create(service)
			if err != nil {
				logrus.Panic(err)
			}
			cb := serviceClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, service.Name, &metaV1.DeleteOptions{}))
		case *coreV1.Secret:
			secret := item.(*coreV1.Secret)
			logrus.Info("Creating Secret: ", secret.Name)
			if secret.Labels["generatedBy"] == "cli" {
				keyName := secret.Labels["keyName"]
				privateKeyPemBlock, publicKeyPemBlock, err := GenerateRSAKeys(1024)
				if err != nil {
					logrus.Panic(err)
				}
				publicKeyByte := pem.EncodeToMemory(publicKeyPemBlock)
				privateByte := pem.EncodeToMemory(privateKeyPemBlock)
				tokenStr, err := SignedToken(keyName, privateByte)
				if err != nil {
					logrus.Panic(err)
				}
				secret.Data = map[string][]byte{
					keyName + ".public.key": publicKeyByte,
					keyName + ".token": []byte(tokenStr),
				}
			}
			_, err := secretClient.Create(secret)
			if err != nil {
				logrus.Panic(err)
			}
			cb := secretClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, secret.Name, &metaV1.DeleteOptions{}))
		case *coreV1.PersistentVolumeClaim:
			pvc := item.(*coreV1.PersistentVolumeClaim)
			if PVType == "Local" {
				pv := &coreV1.PersistentVolume{
					ObjectMeta: metaV1.ObjectMeta{
						Name: "funceasy-mysql-pv-volume",
					},
					Spec:       coreV1.PersistentVolumeSpec{
						PersistentVolumeReclaimPolicy: coreV1.PersistentVolumeReclaimRecycle,
						Capacity: pvc.Spec.Resources.Requests,
						AccessModes: []coreV1.PersistentVolumeAccessMode{
							coreV1.ReadWriteOnce,
						},
						PersistentVolumeSource: coreV1.PersistentVolumeSource{
							HostPath:             &coreV1.HostPathVolumeSource{
								Path: pathOrClass,
							},
						},
					},
				}
				logrus.Info("Creating PV: ", pv.Name)
				_, err := PVClient.Create(pv)
				if err != nil {
					if !errors.IsAlreadyExists(err) {
						logrus.Panic(err)
					}
					logrus.Warn("AlreadyExists PV: ", pv.Name)
				}
				cb := PVClient.Delete
				rollback = append(rollback, util.GenerateDeleteCallback(cb, pv.Name, &metaV1.DeleteOptions{}))
			}
			if PVType == "StorageClass" {
				 pvc.Spec.StorageClassName = &pathOrClass
			}
			logrus.Info("Creating PVC: ", pvc.Name)
			_, err := PVCClient.Create(pvc)
			if err != nil {
				logrus.Panic(err)
			}
			cb := PVCClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, pvc.Name, &metaV1.DeleteOptions{}))
		case *coreV1.ServiceAccount:
			sa := item.(*coreV1.ServiceAccount)
			logrus.Info("Creating ServiceAccount: ", sa.Name)
			_, err := SAClient.Create(sa)
			if err != nil {
				logrus.Panic(err)
			}
			cb := SAClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, sa.Name, &metaV1.DeleteOptions{}))
		case *rbacV1.Role:
			role := item.(*rbacV1.Role)
			logrus.Info("Creating Role: ", role.Name)
			_, err := RoleClient.Create(role)
			if err != nil {
				logrus.Panic(err)
			}
			cb := RoleClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, role.Name, &metaV1.DeleteOptions{}))
		case *rbacV1.RoleBinding:
			rb := item.(*rbacV1.RoleBinding)
			logrus.Info("Creating RoleBinding: ", rb.Name)
			_, err := RBClient.Create(rb)
			if err != nil {
				logrus.Panic(err)
			}
			cb := RBClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, rb.Name, &metaV1.DeleteOptions{}))
		case *v1beta1.CustomResourceDefinition:
			crd := item.(*v1beta1.CustomResourceDefinition)
			logrus.Info("Creating CRD: ", crd.Name)
			_, err := CRDClient.Create(crd)
			if err != nil {
				if !errors.IsAlreadyExists(err) {
					logrus.Panic(err)
				}
				logrus.Warn("AlreadyExists CRD: ", crd.Name)
			}
			cb := CRDClient.Delete
			rollback = append(rollback, util.GenerateDeleteCallback(cb, crd.Name, &metaV1.DeleteOptions{}))
		}
	}
	return nil
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
			logrus.Info("Check Pod Status")
			if pod.Status.Phase == coreV1.PodRunning {
				for _, item := range pod.Status.Conditions {
					if item.Type == coreV1.PodReady && item.Status == coreV1.ConditionTrue {
						logrus.Info("Check Pod Status: Ready")
						done<-true
						result<-"success"
					}
				}
			}
			if pod.Status.Phase == coreV1.PodFailed {
				logrus.Info("Check Pod Status: Failed")
				done<-true
				result<-"failed"
			}
		} else {
			return nil
		}
		return nil
	}
}
