helm delete istio-ingress -n istio-ingress
kubectl delete namespace istio-ingress
helm delete ztunnel -n istio-system
helm delete istio-cni -n istio-system
helm delete istiod -n istio-system
helm delete istio-base -n istio-system
kubectl get crd -oname | grep --color=never 'istio.io' | xargs kubectl delete
kubectl delete namespace istio-system
eksctl delete cluster --name basic-cluster --region us-east-1 --disable-nodegroup-eviction