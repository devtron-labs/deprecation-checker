/*
 * Copyright (c) 2021 Devtron Labs
 *
 * Licensed under the Apache License, Version 2.0 (the "License");
 * you may not use this file except in compliance with the License.
 * You may obtain a copy of the License at
 *
 *    http://www.apache.org/licenses/LICENSE-2.0
 *
 * Unless required by applicable law or agreed to in writing, software
 * distributed under the License is distributed on an "AS IS" BASIS,
 * WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
 * See the License for the specific language governing permissions and
 * limitations under the License.
 *
 */

package pkg

import (
	"testing"
)

const deployment = `
{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "nginx-deployment",
    "labels": {
      "app": "nginx"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "nginx",
            "image": "nginx:1.14.2",
            "ports": [
              {
                "containerPort1": 80
              }
            ]
          }
        ]
      }
    }
  }
}`

const correct_deployment = `
{
  "apiVersion": "apps/v1",
  "kind": "Deployment",
  "metadata": {
    "name": "nginx-deployment",
    "labels": {
      "app": "nginx"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "nginx",
            "image": "nginx:1.14.2",
            "ports": [
              {
                "containerPort": 80
              }
            ]
          }
        ]
      }
    }
  }
}`

const extension_deployment = `
{
  "apiVersion": "extensions/v1beta1",
  "kind": "Deployment",
  "metadata": {
    "name": "nginx-deployment",
    "labels": {
      "app": "nginx"
    }
  },
  "spec": {
    "replicas": 3,
    "selector": {
      "matchLabels": {
        "app": "nginx"
      }
    },
    "rollbackTo": {
       "revision": 12
    },
    "template": {
      "metadata": {
        "labels": {
          "app": "nginx"
        }
      },
      "spec": {
        "containers": [
          {
            "name": "nginx",
            "image": "nginx:1.14.2",
            "ports": [
              {
                "containerPort": 80
              }
            ]
          }
        ]
      }
    }
  }
}`

const svc = `
{
  "apiVersion": "v1",
  "kind": "Service",
  "metadata": {
    "name": "my-service"
  },
  "spec": {
    "selector": {
      "app": "MyApp"
    },
    "ports": [
      {
        "protocol": "TCP",
        "port": 80,
        "targetPort": 9376
      }
    ]
  }
}`

const cm =`
{
  "apiVersion": "v1",
  "kind": "ConfigMap",
  "metadata": {
    "name": "game-demo"
  },
  "data": {
    "player_initial_lives": "3",
    "ui_properties_file_name": "user-interface.properties",
    "game.properties": "enemy.types=aliens,monsters\nplayer.maximum-lives=5    \n",
    "user-interface.properties": "color.good=purple\ncolor.bad=yellow\nallow.textmode=true \n"
  }
}`

const secret = `
{
  "apiVersion": "v1",
  "kind": "Secret",
  "metadata": {
    "name": "bootstrap-token-5emitj",
    "namespace": "kube-system"
  },
  "type": "bootstrap.kubernetes.io/token",
  "data": {
    "auth-extra-groups": "c3lzdGVtOmJvb3RzdHJhcHBlcnM6a3ViZWFkbTpkZWZhdWx0LW5vZGUtdG9rZW4=",
    "expiration": "MjAyMC0wOS0xM1QwNDozOToxMFo=",
    "token-id": "NWVtaXRq",
    "token-secret": "a3E0Z2lodnN6emduMXAwcg==",
    "usage-bootstrap-authentication": "dHJ1ZQ==",
    "usage-bootstrap-signing": "dHJ1ZQ=="
  }
}`

const secret_stringdata = `
{
  "apiVersion": "v1",
  "kind": "Secret",
  "metadata": {
    "name": "bootstrap-token-5emitj",
    "namespace": "kube-system"
  },
  "type": "bootstrap.kubernetes.io/token",
  "stringData": {
    "auth-extra-groups": "system:bootstrappers:kubeadm:default-node-token",
    "expiration": "2020-09-13T04:39:10Z",
    "token-id": "5emitj",
    "token-secret": "kq4gihvszzgn1p0r",
    "usage-bootstrap-authentication": "true",
    "usage-bootstrap-signing": "true"
  }
}`

func TestDownloadFile(t *testing.T) {
	type args struct {
		releaseVersion string
		object string
	}
	tests := []struct {
		name    string
		args    args
		wantErr bool
	}{
		{
			name: "Positive - Test deployment",
			args: args{releaseVersion: "1.16", object: correct_deployment},
			wantErr: false,
		},
		{
			name: "Negative - Test deployment",
			args: args{releaseVersion: "1.16", object: deployment},
			wantErr: true,
		},
		{
			name: "Positive - Test Service",
			args: args{releaseVersion: "1.20", object: svc},
			wantErr: false,
		},
		{
			name: "Positive - Test deployment extension, handled via apps/v1",
			args: args{releaseVersion: "1.18", object: extension_deployment},
			wantErr: false,
		},
		{
			name: "Positive - Test deployment extension",
			args: args{releaseVersion: "1.16", object: extension_deployment},
			wantErr: false,
		},
		{
			name: "Positive - Test configmap",
			args: args{releaseVersion: "1.17", object: cm},
			wantErr: false,
		},
		{
			name: "Positive - Test secret",
			args: args{releaseVersion: "1.17", object: secret},
			wantErr: false,
		},
		{
			name: "Positive - Test secret stringdata",
			args: args{releaseVersion: "1.17", object: secret_stringdata},
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			kc := NewKubeCheckerImpl()
			var err error
			err = kc.LoadFromUrl(tt.args.releaseVersion, false)
			err = kc.ValidateJson(tt.args.object, tt.args.releaseVersion)
			//fmt.Println(string(got))
			if (err != nil) != tt.wantErr {
				t.Errorf("DownloadFile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
		})
	}
}

func Test_compareVersion(t *testing.T) {
	type args struct {
		first  string
		second string
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.extensions.v1beta1.Deployment",
				second: "io.k8s.api.apps.v1.Deployment",
			},
			want: "io.k8s.api.apps.v1.Deployment",
			wantErr: false,
		},
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.extensions.v2beta1.Deployment",
				second: "io.k8s.api.apps.v1.Deployment",
			},
			want: "io.k8s.api.apps.v1.Deployment",
			wantErr: false,
		},
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.apps.v2beta1.Deployment",
				second: "io.k8s.api.apps.v1.Deployment",
			},
			want: "io.k8s.api.apps.v2beta1.Deployment",
			wantErr: false,
		},
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.extensions.v1alpha1.Deployment",
				second: "io.k8s.api.extensions.v1beta1.Deployment",
			},
			want: "io.k8s.api.extensions.v1beta1.Deployment",
			wantErr: false,
		},
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.extensions.v1beta1.Deployment",
				second: "io.k8s.api.extensions.v1beta2.Deployment",
			},
			want: "io.k8s.api.extensions.v1beta2.Deployment",
			wantErr: false,
		},
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.extensions.v1beta2.Deployment",
				second: "io.k8s.api.extensions.v1beta1.Deployment",
			},
			want: "io.k8s.api.extensions.v1beta2.Deployment",
			wantErr: false,
		},
		{
			name: "compare deployment",
			args: args{
				first:  "io.k8s.api.apps.v2.Deployment",
				second: "io.k8s.api.apps.v1.Deployment",
			},
			want: "io.k8s.api.apps.v2.Deployment",
			wantErr: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := compareVersion(tt.args.first, tt.args.second)
			if (err != nil) != tt.wantErr {
				t.Errorf("compareVersion() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("compareVersion() got = %v, want %v", got, tt.want)
			}
		})
	}
}