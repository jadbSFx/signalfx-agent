package kubernetes

import (
	"io/ioutil"
	"reflect"
	"testing"
	"time"

	"github.com/kr/pretty"
	"github.com/signalfx/neo-agent/neotest"
	"github.com/signalfx/neo-agent/plugins"
	"github.com/signalfx/neo-agent/services"
	"github.com/spf13/viper"
)

// Test_load verifies that the raw Kubelet JSON transforms into the expected Go
// struct.
func Test_load(t *testing.T) {
	podsJSON, err := ioutil.ReadFile("testdata/pods.json")
	if err != nil {
		t.Fatal("failed loading pods.json")
	}

	loadedPods := &pods{}
	neotest.LoadJSON(t, "testdata/pods-loaded.json", loadedPods)

	type args struct {
		body []byte
	}
	tests := []struct {
		name    string
		args    args
		want    *pods
		wantErr bool
	}{
		{"load failed", args{[]byte("invalid")}, nil, true},
		{"load succeded", args{podsJSON}, loadedPods, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := load(tt.args.body)
			if (err != nil) != tt.wantErr {
				t.Errorf("load() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				pretty.Ldiff(t, got, tt.want)
				t.Error("Differences detected")
			}
		})
	}
}

func TestKubernetes_doMap(t *testing.T) {
	var running, nonRunningContainer *pods
	var expected services.ServiceInstances
	neotest.LoadJSON(t, "testdata/pods.json", &running)
	neotest.LoadJSON(t, "testdata/pods.json", &nonRunningContainer)
	neotest.LoadJSON(t, "testdata/2-discovered.json", &expected)

	// Make container non-running
	nonRunningContainer.Items = nonRunningContainer.Items[:1]
	containerState := nonRunningContainer.Items[0].Status.ContainerStatuses[0].State
	containerState["waiting"] = struct{}{}
	delete(containerState, "running")

	// Set time.Now() to fixed value.
	now = neotest.FixedTime
	defer func() { now = time.Now }()

	config := viper.New()
	config.Set("hosturl", "unused")
	k, err := NewKubernetes("kubernetes", config)
	if err != nil {
		t.Fatal(err)
	}

	type fields struct {
		Plugin  plugins.Plugin
		hostURL string
	}
	type args struct {
		sis  services.ServiceInstances
		pods *pods
	}
	tests := []struct {
		name     string
		instance *Kubernetes
		args     args
		want     services.ServiceInstances
		wantErr  bool
	}{
		{"zero instances", k, args{nil, &pods{}}, nil, false},
		{"two kubernetes only instances", k, args{nil, running}, expected, false},
		{"container status is not running", k, args{nil, nonRunningContainer}, nil, false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {

			got, err := k.doMap(tt.args.sis, tt.args.pods)
			if (err != nil) != tt.wantErr {
				t.Errorf("Kubernetes.doMap() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				pretty.Ldiff(t, got, tt.want)
				t.Error("Differences detected")
			}
		})
	}
}