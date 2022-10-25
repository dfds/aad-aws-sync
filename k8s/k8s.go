package k8s

import (
	"context"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/aws"
	"gopkg.in/yaml.v2"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/env"
)

type RoleMapping struct {
	// RoleARN is the AWS Resource Name of the role. (e.g., "arn:aws:iam::000000000000:role/Foo").
	RoleARN string `json:"rolearn,omitempty" yaml:"rolearn,omitempty"`

	// ManagedBy is a custom field(not part of the original spec) that is used for detecting objects managed by aad-aws-sync
	ManagedBy string `json:"managedby,omitempty" yaml:"managedby,omitempty"`

	// LastUpdated is a custom field(not part of the original spec)
	LastUpdated string `json:"lastupdated,omitempty" yaml:"lastupdated,omitempty"`

	// CreatedAt  is a custom field(not part of the original spec)
	CreatedAt string `json:"createdat,omitempty" yaml:"createdat,omitempty"`

	// Username is the username pattern that this instances assuming this
	// role will have in Kubernetes.
	Username string `json:"username" yaml:"username"`

	// Groups is a list of Kubernetes groups this role will authenticate
	// as (e.g., `system:masters`). Each group name can include placeholders.
	Groups []string `json:"groups" yaml:"groups"`
}

type LoadRoleMapResponse struct {
	Mappings  []*RoleMapping
	ConfigMap *v1.ConfigMap
}

func (l *LoadRoleMapResponse) GetMappingByArn(val string) *RoleMapping {
	for _, m := range l.Mappings {
		if val == m.RoleARN {
			return m
		}
	}

	return nil
}

func (r *RoleMapping) ManagedByThis() bool {
	if r.ManagedBy == "aad-aws-sync" {
		return true
	} else {
		return false
	}
}

func (r *RoleMapping) ContainsGroup(val string) bool {
	for _, r := range r.Groups {
		if val == r {
			return true
		}
	}

	return false
}

func LoadAwsAuthMapRoles() (*LoadRoleMapResponse, error) {
	client, err := getK8sClient()
	if err != nil {
		return nil, err
	}

	cm, err := client.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "aws-auth", metav1.GetOptions{})
	if err != nil {
		return nil, err
	}

	mapRolesRaw := cm.Data["mapRoles"]

	var mappings []*RoleMapping

	err = yaml.Unmarshal([]byte(mapRolesRaw), &mappings)
	if err != nil {
		return nil, err
	}

	return &LoadRoleMapResponse{
		Mappings:  mappings,
		ConfigMap: cm,
	}, nil
}

func UpdateAwsAuthMapRoles(cm *v1.ConfigMap) error {
	client, err := getK8sClient()
	if err != nil {
		return err
	}

	_, err = client.CoreV1().ConfigMaps("kube-system").Update(context.TODO(), cm, metav1.UpdateOptions{})
	return err
}

func getK8sClient() (*kubernetes.Clientset, error) {
	config, err := clientcmd.BuildConfigFromFlags("", env.GetString("KUBECONFIG", ""))
	if err != nil {
		return nil, err
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return client, nil
}

func GenerateAwsAuthMapRolesObject(entries map[string]aws.SsoRoleMapping) {
	for _, entry := range entries {
		obj := fmt.Sprintf(`
    - rolearn: arn:aws:iam::%s:role/%s
      username: %s:sso-{{SessionName}}
      groups:
        - DFDS-ReadOnly
        - %s`, entry.AccountId, entry.RoleName, entry.RootId, entry.RootId)

		fmt.Print(obj)
	}
	fmt.Print("\n")
}
