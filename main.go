package main

import (
	"bytes"
	"fmt"
	"os"
	"strings"

	yaml "gopkg.in/yaml.v2"

	"github.com/pkg/errors"
	"github.com/spf13/cobra"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/ec2"
)

type node struct {
	Host       string
	IP         string
	InternalIP string
}

type nodes struct {
	Etcd    []node
	Master  []node
	Worker  []node
	Ingress []node `yaml:",omitempty"`
	Storage []node `yaml:",omitempty"`
}

type multiErr []error

func (me multiErr) Error() string {
	if me == nil {
		return ""
	}
	b := &bytes.Buffer{}
	fmt.Fprintln(b)
	for _, err := range me {
		fmt.Fprintf(b, "- %v\n", err)
	}
	return b.String()
}

func main() {
	var validRoles = map[string]bool{"etcd": true, "master": true, "worker": true, "ingress": true, "storage": true}
	var awsRegions []string
	var awsKETRoleTag string
	var awsCmd = &cobra.Command{
		Use:   "aws",
		Short: "Use nodes that are running on AWS",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(awsRegions) < 1 {
				return errors.New("At least one AWS region must be provided")
			}
			var verrs multiErr
			if os.Getenv("AWS_ACCESS_KEY_ID") == "" {
				verrs = append(verrs, errors.New("AWS_ACCESS_KEY_ID environment variable must be set"))
			}
			if os.Getenv("AWS_SECRET_ACCESS_KEY") == "" {
				verrs = append(verrs, errors.New("AWS_SECRET_ACCESS_KEY environment variable must be set"))
			}
			if len(verrs) > 0 {
				return errors.Wrapf(verrs, "missing AWS environment variables")
			}
			var ns nodes
			for _, reg := range awsRegions {
				awsSession := session.Must(session.NewSession(&aws.Config{Region: aws.String(reg)}))
				svc := ec2.New(awsSession)
				params := ec2.DescribeInstancesInput{
					Filters: []*ec2.Filter{
						&ec2.Filter{
							Name:   aws.String(fmt.Sprintf("tag:%s", awsKETRoleTag)),
							Values: aws.StringSlice([]string{"*"}),
						},
					},
				}
				res, err := svc.DescribeInstances(&params)
				if err != nil {
					return errors.Wrap(err, "failed to get instances from AWS")
				}

				for _, r := range res.Reservations {
					if len(r.Instances) > 1 {
						return errors.Wrap(err, "multiple instances found in single reservation...")
					}
					i := *r.Instances[0]
					node, err := getNodeMetadataAWS(i)
					if err != nil {
						return errors.Wrapf(err, "instance %q is missing required metadata", *i.InstanceId)
					}

					roles, err := getRolesFromAWS(awsKETRoleTag, i.Tags)
					if err != nil {
						return errors.Wrapf(err, "failed to get roles for node %q", *i.InstanceId)
					}
					for _, role := range roles {
						r := strings.ToLower(role)
						if _, ok := validRoles[r]; !ok {
							return errors.Wrapf(err, "instance %q has an invalid role %q", *i.InstanceId, r)
						}
						switch r {
						case "etcd":
							ns.Etcd = append(ns.Etcd, node)
						case "master":
							ns.Master = append(ns.Master, node)
						case "worker":
							ns.Worker = append(ns.Worker, node)
						case "ingress":
							ns.Ingress = append(ns.Ingress, node)
						case "storage":
							ns.Storage = append(ns.Storage, node)
						default:
							return errors.Errorf("instance %q has invalid role tag %q", *i.InstanceId, r)
						}
					}
				}
			}
			d, err := yaml.Marshal(&ns)
			if err != nil {
				return errors.Wrap(err, "error marshaling nodes")
			}
			fmt.Print(string(d))
			return nil
		},
	}
	awsCmd.Flags().StringSliceVar(&awsRegions, "region", []string{}, "regions to be used")
	awsCmd.Flags().StringVar(&awsKETRoleTag, "role-tag", "KetRole", "the AWS EC2 tag that contains the machine's role")

	var rootCmd = &cobra.Command{Use: "kcp"}
	rootCmd.AddCommand(awsCmd)

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func getNodeMetadataAWS(i ec2.Instance) (node, error) {
	var merr multiErr
	if i.PrivateDnsName == nil {
		merr = append(merr, errors.New("instance does not have a private DNS name"))
	}
	if i.PublicIpAddress == nil {
		merr = append(merr, errors.New("instance does not have a public IP address"))
	}
	if i.PrivateIpAddress == nil {
		merr = append(merr, errors.New("instance does not have a private IP address"))
	}
	if len(merr) > 0 {
		return node{}, merr
	}
	return node{
		Host:       *i.PrivateDnsName,
		IP:         *i.PublicIpAddress,
		InternalIP: *i.PrivateIpAddress,
	}, nil
}

func getRolesFromAWS(roleTag string, tags []*ec2.Tag) ([]string, error) {
	for _, t := range tags {
		if t.Key == nil {
			return nil, errors.New("tag key is nil")
		}
		if roleTag == *t.Key {
			if t.Value == nil {
				return nil, errors.Errorf("value for tag %q is nil", *t.Key)
			}
			return strings.Split(*t.Value, ","), nil
		}
	}
	return nil, fmt.Errorf("tag %q not found", roleTag)
}
