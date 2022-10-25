package k8s

import (
	"context"
	"fmt"
	"go.dfds.cloud/aad-aws-sync/aws"
	"gopkg.in/yaml.v2"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/utils/env"
	"log"
)

type RoleMapping struct {
	// RoleARN is the AWS Resource Name of the role. (e.g., "arn:aws:iam::000000000000:role/Foo").
	RoleARN string `json:"rolearn,omitempty" yaml:"rolearn,omitempty"`

	// ManagedBy is a custom field(not part of the original spec) that is used for detecting objects managed by aad-aws-sync
	ManagedBy string `json:"managedby,omitempty" yaml:"managedby,omitempty"`

	// Username is the username pattern that this instances assuming this
	// role will have in Kubernetes.
	Username string `json:"username" yaml:"username"`

	// Groups is a list of Kubernetes groups this role will authenticate
	// as (e.g., `system:masters`). Each group name can include placeholders.
	Groups []string `json:"groups" yaml:"groups"`
}

func (r *RoleMapping) ManagedByThis() bool {
	if r.ManagedBy == "aad-aws-sync" {
		return true
	} else {
		return false
	}
}

func LoadAwsAuthMap() {
	config, err := clientcmd.BuildConfigFromFlags("", env.GetString("KUBECONFIG", ""))
	if err != nil {
		log.Fatal(err)
	}

	client, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	cm, err := client.CoreV1().ConfigMaps("kube-system").Get(context.TODO(), "aws-auth", metav1.GetOptions{})
	if err != nil {
		log.Fatal(err)
	}

	mapRolesRaw := cm.Data["mapRoles"]

	var mappings []RoleMapping

	err = yaml.Unmarshal([]byte(mapRolesRaw), &mappings)
	if err != nil {
		log.Fatal(err)
	}

	for _, mapping := range mappings {
		if mapping.ManagedByThis() {
			fmt.Println(mapping)
		}
	}
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
