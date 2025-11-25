## 1. `operator-sdk init`
```bash
# operator-sdk init
$ operator-sdk init --domain=cloudclub.com --repo=github.com/cloud-club/08th-k8s-operator/monitoring-been
INFO[0000] Writing kustomize manifests for you to edit... 
INFO[0000] Writing scaffold for you to edit...          
INFO[0000] Get controller runtime:
$ go get sigs.k8s.io/controller-runtime@v0.21.0 
INFO[0005] Update dependencies:
$ go mod tidy           
Next: define a resource with:
$ operator-sdk create api
```


## 2. `operator-sdk create api`
```bash
$ operator-sdk create api --group cleanup --version v1alpha1 --kind CleanupPolicy --resource --controller
```