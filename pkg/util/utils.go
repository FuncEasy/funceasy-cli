package util

import (
	"github.com/funceasy/funceasy-cli/pkg/util/terminal"
	"github.com/sirupsen/logrus"
	metaV1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"strings"
	"time"
)

func ParseK8sYaml(fileByte []byte) ([]runtime.Object, error) {
	yamlFileSplits := SplitK8sYaml(fileByte)
	objectList := make([]runtime.Object, 0, len(yamlFileSplits))
	for _, fileStr := range yamlFileSplits {
		if fileStr == "\n" || fileStr == "" {
			continue
		}
		decode := scheme.Codecs.UniversalDeserializer().Decode
		obj, _, err := decode([]byte(fileStr), nil, nil)
		if err != nil {
			return nil, err
		}
		objectList = append(objectList, obj)
	}
	return objectList, nil
}

func SplitK8sYaml(fileByte []byte) []string {
	readFileAsString := string(fileByte[:])
	yamlFileSplits := strings.Split(readFileAsString, "---")
	return yamlFileSplits
}

func GenerateDeleteCallback(function func(string, *metaV1.DeleteOptions) error, name string, options *metaV1.DeleteOptions) func() error {
	return func() error {
		err := function(name, options)
		if err != nil {
			return err
		}
		return nil
	}
}

func GenerateUpdateCallback()  {
	
}

func Rollback(rollbackList []func() error, t *terminal.Terminal) {
	t.PrintWarnOneLine("Start RollBack: %d Tasks", len(rollbackList))
	t.LineEnd()
	for _, cb := range rollbackList {
		err := cb()
		if err != nil {
			t.PrintErrorOneLineWithPanic(err)
		}
	}
	t.PrintSuccessOneLine("RollBack Complete: %d Tasks", len(rollbackList))
	t.LineEnd()
}

func PollingCheck(do func(result chan string, done chan bool) error, interval time.Duration ,result chan string, done chan bool) {
	for {
		select {
		case <-done:
			return
		case <-time.After(interval):
			go func() {
				if err := do(result, done); err != nil {
					logrus.Error(err)
					result<-"failed"
					return
				}
			}()
		}
	}
}
