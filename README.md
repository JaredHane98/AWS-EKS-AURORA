This repository sets up an EKS Cluster with an Aurora Serverless V2 PostgreSQL database. It assumes that you have the following tools installed: [Cilium CLI](https://kubernetes.io/docs/reference/kubectl/), [EKSCTL](https://eksctl.io/installation/), [kubectl](https://kubernetes.io/docs/reference/kubectl/), [docker](https://docs.docker.com/engine/install/). Additionally you must have the necessary permissions to create the database. You should also be aware these resources may incur costs.


First off clone the repository if you already haven’t.

```bash
git clone https://github.com/JaredHane98/AWS-EKS-AURORA.git
```

# Deploying the cluster

Nagivate to  the CreateEKSCluster directory

```bash
cd ./AWS-EKS-AURORA/CreateEKSCluster
```

Next, create the cluster. Make sure to take note of the VPC created by the cluster, as it will be used in the subsequent steps.

```bash
eksctl create cluster -f cluster-launch.yml
```

Check the progress of the VPC using the CLI. You can also use AWS console.

```bash
aws ec2 describe-vpcs
{
            "CidrBlock": "192.168.0.0/16",
            "DhcpOptionsId": DHPC_OPTIONS_ID,
            "State": "available",
            "VpcId": VPC_ID,
            "OwnerId": ACCOUNT_ID,
            "InstanceTenancy": "default",
            "CidrBlockAssociationSet": [
                {
                    "AssociationId": VPC_ASSOCIATION_ID,
                    "CidrBlock": "192.168.0.0/16",
                    "CidrBlockState": {
                        "State": "associated"
                    }
                }
            ],
            "IsDefault": false,
            "Tags": [
                {
                    "Key": "aws:cloudformation:stack-name",
                    "Value": "eksctl-db-cluster-1-cluster"
                },
```

# Deploying Aurora Serverless V2 RDS Database

Create another window and navigate to the CreateAuroraCDK directory.

```bash
cd CreateAuroraCDK
```

Now we need to create a few enviromental variables for the CDK

```bash
export AWS_REGION=YOUR-REGION
export VPC_ID=vpc-YOUR-VPC_ID
export AWS_ACCOUNT_ID=$(aws sts get-caller-identity --query Account --output text)
```

Then deploy the resources to AWS

```bash
cdk deploy
```

# Installing Cilium

Return to the window where the cluster is being created and wait for the process to complete. Once it’s finished, install Cilium using the following commands.

```bash
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.1.0/config/crd/standard/gateway.networking.k8s.io_gatewayclasses.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.1.0/config/crd/standard/gateway.networking.k8s.io_gateways.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.1.0/config/crd/standard/gateway.networking.k8s.io_httproutes.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.1.0/config/crd/standard/gateway.networking.k8s.io_referencegrants.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.1.0/config/crd/standard/gateway.networking.k8s.io_grpcroutes.yaml
kubectl apply -f https://raw.githubusercontent.com/kubernetes-sigs/gateway-api/v1.1.0/config/crd/experimental/gateway.networking.k8s.io_tlsroutes.yaml
git clone https://github.com/cilium/cilium.git
cd cilium
cilium install --chart-directory ./install/kubernetes/cilium --set kubeProxyReplacement=true --set gatewayAPI.enabled=true
```

Validate Cilium has  been properly installed

```bash
cilium status --wait
```

Run a network test with Cilium to check network connectivity

```bash
cilium connectivity test
```

# Setup IAM Service Account

With the cluster set up and Cilium in place, we can now create a service account for the deployment. Note that the upcoming steps will require the Secret ARN created by the Aurora instance.


Create an IAM OIDC identity provider for our cluster

```bash
cd ..
cluster_name=db-cluster-1
oidc_id=$(aws eks describe-cluster --name $cluster_name --query "cluster.identity.oidc.issuer" --output text | cut -d '/' -f 5)
echo $oidc_id
aws iam list-open-id-connect-providers | grep $oidc_id | cut -d "/" -f4
eksctl utils associate-iam-oidc-provider --cluster $cluster_name --approve
```

Create an enviromental variable for the RDS_SECRET_ARN

```bash
export RDS_SECRET_ARN=SECRET_ARN_FROM_CDK
```

Create an IAM policy file

```bash
cat >iam-policy.json <<EOF
{
    "Version": "2012-10-17",
    "Statement": [
        {
            "Sid": "SecretsManagerDbCredentialsAccess",
            "Effect": "Allow",
            "Action": [
                "secretsmanager:GetSecretValue"
            ],
            "Resource": "$RDS_SECRET_ARN"
        }
    ]
}
EOF
```

Create the IAM Policy

```bash
aws iam create-policy --policy-name db-policy-1 --policy-document file://iam-policy.json
```

Create the service account. Replace the --attach-policy-arn with the one from the previous step.

```bash
eksctl create iamserviceaccount --name db-service-account-1 --namespace default --cluster db-cluster-1 \
 --attach-policy-arn arn:aws:iam::111122223333:policy/my-policy --approve
```

# Create the RDS Table

Instead of launching an EC2 instance within the VPC, you can use the RDS Query Editor to create a table. Log in to the editor using the RDS_SECRET_ARN and RDS_DATABASE_NAME provided in the CDK outputs. Then, create a table using the following command.

```sql
create table EmployeeTable (
  id uuid PRIMARY KEY,
  first_name text,
  last_name text,
  field text,
  start_time date,
  dob date,
  salary INT
);
```

# Create the containers

Go back to the console and navigate to the EKSApp directory.

```bash
cd EKSApp
```

Build the container using docker

```bash
docker build --tag eks-app .
```

The next steps depend on your choice of repository. I personally use [AWS ECR](https://aws.amazon.com/ecr/).

```bash
docker tag eks-app ACCOUNT_ID.dkr.ecr.us-east-1.amazomaws.com/aurora/containers:eks-app
docker push ACCOUNT_ID.dkr.ecr.us-east-1.amazomaws.com/aurora/containers:eks-app
```

Regardless of your choice you must remember the image URL.

# Deploying the pods

Navigate back to the CreateEKSCluster directory.

```bash
cd ..
```

Create a few enviromental variables using the CDK output and container image URL.

```bash
export CONTAINER_IMAGE_URL=01234567912.dkr.ecr.us-east-1.amazomaws.com/aurora/containers:eks-app
export RDS_SECRET=RDS_SECRET_FROM_CDK
```

Create a deployment file.

```bash
cat >deployment.yml << EOF
---
apiVersion: v1
kind: Service                    # Type of kubernetes resource
metadata:
  name: eks-app                  # Name of the resource
spec:
  ports:                         # Take incoming HTTP requests on port 9090 and forward them to the targetPort of 8080
  - name: http
    port: 8080
  selector:
    app: eks-app         # Map any pod with label app1
---
apiVersion: apps/v1
kind: Deployment                 # Type of Kubernetes resource
metadata:
  name: eks-app                    # Name of the Kubernetes resource
spec:
  replicas: 3                    # Number of pods to run at any given time
  selector:
    matchLabels:
      app: eks-app                # This deployment applies to any Pods matching the specified label
  template:                      # This deployment will create a set of pods using the configurations in this template
    metadata:
      labels:                    # The labels that will be applied to all of the pods in this deployment
        app: eks-app 
    spec:                        # Spec for the container which will run in the Pod
      serviceAccountName: db-service-account-1
      containers:
      - name: eks-app
        image: $CONTAINER_IMAGE_URL
        imagePullPolicy: Always   # only attempts to pull if not local
        ports:
          - containerPort: 8080   # Should match the port number that the Go application listens on
        livenessProbe:            # To check the health of the Pod
          httpGet:
            path: /health
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 5
          periodSeconds: 15
          timeoutSeconds: 5
        readinessProbe:          # To check if the Pod is ready to serve traffic or not
          httpGet:
            path: /readiness
            port: 8080
            scheme: HTTP
          initialDelaySeconds: 5
          timeoutSeconds: 1
        resources:
          requests:
            cpu: 300m
            memory: 128Mi
          limits:
            cpu: 500m
            memory: 256Mi 
        env:
        - name: RDS_TABLE_NAME
          value: EmployeeTable
        - name: RDS_SECRET
          value: $RDS_SECRET
      affinity:
        nodeAffinity:
          requiredDuringSchedulingIgnoredDuringExecution:
            nodeSelectorTerms:
            - matchExpressions:
              - key: "kubernetes.io/os"
                operator: In
                values:
                - linux
              - key: "kubernetes.io/arch"
                operator: In
                values:
                - amd64
EOF
```

Now launch the deployment and gateway.

```bash
kubectl apply -f deployment.yml
kubectl apply -f gateway.yml
```

Check the pods status

```bash
kubectl get pods -o wide
```

They should look similarly to this.

```bash
NAME                       READY   STATUS    RESTARTS   AGE   IP                NODE                             NOMINATED NODE   READINESS GATES
eks-app-867567dc4b-b5v4m   1/1     Running   0          12s   192.168.109.107   ip-192-168-121-59.ec2.internal   <none>           <none>
eks-app-867567dc4b-dkw67   1/1     Running   0          12s   192.168.99.113    ip-192-168-121-59.ec2.internal   <none>           <none>
eks-app-867567dc4b-l7c6g   1/1     Running   0          12s   192.168.120.32    ip-192-168-121-59.ec2.internal   <none>           <none>
```

Get the assigned URL

```bash
GATEWAY=$(kubectl get gateway basic-gateway -o jsonpath='{.status.addresses[0].value}')
```

Add an employee to the registry

```markup
curl --header "Content-Type: application/json" --request POST --data '{"id": "ebae8ff2-2e25-49b1-b7a6-3d6f5e8a20bd", "first_name": "Jared", "last_name": "Hane", "sector": "Programmer", "start_time": "2024-1-27", "dob": "1996-10-23", "salary": 134903 }' http://$GATEWAY/employee
```

Get an employee

```bash
curl --request GET http://$GATEWAY/employee/ebae8ff2-2e25-49b1-b7a6-3d6f5e8a20bd
```

Update an employee

```bash
curl --header "Content-Type: application/json" --request PUT --data '{"salary": 150000}' http://$GATEWAY/employee/ebae8ff2-2e25-49b1-b7a6-3d6f5e8a20bd
```

Verify the salary has been updated

```bash
curl --request GET http://$GATEWAY/employee/ebae8ff2-2e25-49b1-b7a6-3d6f5e8a20bd
```

Remove an employee

```bash
curl --request DELETE http://$GATEWAY/employee/ebae8ff2-2e25-49b1-b7a6-3d6f5e8a20bd
```

# Cleaning up

Navigate to the CreateAuroraCDK directory and run

```bash
cdk destroy
```

Navigate to the CreateEKSCluster and run

```bash
eksctl delete cluster -f cluster-launch.yml --disable-nodegroup-eviction
```

You may also have to manually delete the VPC and associated load balancer in the AWS console.

# Things to add

* Horizontal scaling
* Cluster node scaling
* Stress test using K6 or any other platform
* Improved Aurora Scaling
* Create a job initialize the database
* TLS Certification

# Final words

There’s another project in the directory named CreateDatabase. It generates over 4,000 random entries that you can use.
