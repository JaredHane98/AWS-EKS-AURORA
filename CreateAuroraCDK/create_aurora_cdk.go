package main

import (
	"os"

	"github.com/aws/aws-cdk-go/awscdk/v2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsec2"
	"github.com/aws/aws-cdk-go/awscdk/v2/awsrds"

	// "github.com/aws/aws-cdk-go/awscdk/v2/awssqs"
	"github.com/aws/constructs-go/constructs/v10"
	"github.com/aws/jsii-runtime-go"
)

type CreateAuroraCdkStackProps struct {
	awscdk.StackProps
}

func getPrivateIsolatedSubnets(vpc awsec2.IVpc) []awsec2.ISubnet {
	var privateIsolatedSubnets []awsec2.ISubnet
	for _, subnet := range *vpc.PrivateSubnets() {
		if subnet.AvailabilityZone() != nil && subnet.Ipv4CidrBlock() != nil {
			privateIsolatedSubnets = append(privateIsolatedSubnets, subnet)
		}
	}
	return privateIsolatedSubnets
}

func NewCreateAuroraCdkStack(scope constructs.Construct, id string, props *CreateAuroraCdkStackProps) awscdk.Stack {
	var sprops awscdk.StackProps
	if props != nil {
		sprops = props.StackProps
	}
	stack := awscdk.NewStack(scope, &id, &sprops)

	vpcID := os.Getenv("VPC_ID")
	if vpcID == "" {
		panic("VPC_ID environment variable is not set")
	}

	//get prexisting VPC
	vpc := awsec2.Vpc_FromLookup(stack, jsii.String("VPC"), &awsec2.VpcLookupOptions{
		VpcId: jsii.String(vpcID),
	})

	clusterSg := awsec2.NewSecurityGroup(stack, jsii.String("AuroraClusterSecurityGroup"), &awsec2.SecurityGroupProps{
		Vpc:              vpc,
		Description:      jsii.String("Security group for Aurora PostgreSQL cluster"),
		AllowAllOutbound: jsii.Bool(true),
	})
	// Allow any IPv4 & any port. Fine for now, but bad for production
	clusterSg.AddIngressRule(
		awsec2.Peer_AnyIpv4(),
		awsec2.Port_TcpRange(jsii.Number(0), jsii.Number(65535)),
		jsii.String("Allow PostgreSQL traffic"),
		jsii.Bool(true),
	)

	privateIsolatedSubnets := getPrivateIsolatedSubnets(vpc)

	cluster := awsrds.NewDatabaseCluster(stack, jsii.String("AuroraClusterV2"), &awsrds.DatabaseClusterProps{
		Engine: awsrds.DatabaseClusterEngine_AuroraPostgres(&awsrds.AuroraPostgresClusterEngineProps{
			Version: awsrds.AuroraPostgresEngineVersion_VER_16_1(),
		}),
		ClusterIdentifier: jsii.String("AuroraDBCluster2"),
		Writer: awsrds.ClusterInstance_ServerlessV2(jsii.String("writer"), &awsrds.ServerlessV2ClusterInstanceProps{
			PubliclyAccessible: jsii.Bool(false),
		}),
		DefaultDatabaseName:     jsii.String("DefaultDatabase"),
		ServerlessV2MinCapacity: jsii.Number(0.5),
		ServerlessV2MaxCapacity: jsii.Number(5),
		RemovalPolicy:           awscdk.RemovalPolicy_DESTROY,
		Vpc:                     vpc,
		EnableDataApi:           jsii.Bool(true), // only used to create the table
		VpcSubnets: &awsec2.SubnetSelection{
			Subnets: &privateIsolatedSubnets,
		},
		SecurityGroups: &[]awsec2.ISecurityGroup{clusterSg},
	})

	awscdk.NewCfnOutput(stack, jsii.String("RdsSecretArn"), &awscdk.CfnOutputProps{
		Value: cluster.Secret().SecretArn(),
	})
	awscdk.NewCfnOutput(stack, jsii.String("RdsDatabaseName"), &awscdk.CfnOutputProps{
		Value: cluster.NewCfnProps().DatabaseName,
	})
	awscdk.NewCfnOutput(stack, jsii.String("RdsSecret"), &awscdk.CfnOutputProps{
		Value: cluster.Secret().SecretName(),
	})

	return stack
}

func main() {
	defer jsii.Close()

	app := awscdk.NewApp(nil)

	NewCreateAuroraCdkStack(app, "CreateAuroraCdkStack", &CreateAuroraCdkStackProps{
		awscdk.StackProps{
			Env: env(),
		},
	})

	app.Synth(nil)
}

// env determines the AWS environment (account+region) in which our stack is to
// be deployed. For more information see: https://docs.aws.amazon.com/cdk/latest/guide/environments.html
func env() *awscdk.Environment {

	awsRegion := os.Getenv("AWS_REGION")
	awsAccountId := os.Getenv("AWS_ACCOUNT_ID")

	if awsRegion == "" || awsAccountId == "" {
		panic("AWS_REGION and AWS_ACCOUNT_ID env are not set")
	}

	return &awscdk.Environment{
		Account: jsii.String(awsAccountId),
		Region:  jsii.String(awsRegion),
	}
}
