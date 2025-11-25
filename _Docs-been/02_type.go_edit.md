## 1. CRD 설계
`api/v1alpha1/cleanuppolicy_types.go` 파일을 수정

```go
package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CleanupPolicySpec defines the desired state of CleanupPolicy
type CleanupPolicySpec struct {
	// Schedule in Cron format for cleanup execution (default: daily at 2 AM)
	// +kubebuilder:default="0 2 * * *"
	Schedule string `json:"schedule,omitempty"`

	// DryRun mode - if true, only log actions without actual deletion
	// +kubebuilder:default=true
	DryRun bool `json:"dryRun,omitempty"`

	// RequireApproval - if true, mark resources for deletion but wait for manual approval
	// +kubebuilder:default=false
	RequireApproval bool `json:"requireApproval,omitempty"`

	// Namespaces to include (empty means all namespaces)
	// +optional
	IncludeNamespaces []string `json:"includeNamespaces,omitempty"`

	// Namespaces to exclude from cleanup
	// +kubebuilder:default={"kube-system","kube-public","kube-node-lease"}
	ExcludeNamespaces []string `json:"excludeNamespaces,omitempty"`

	// Pod cleanup policies
	// +optional
	PodPolicies *PodCleanupPolicy `json:"podPolicies,omitempty"`

	// PersistentVolume cleanup policies
	// +optional
	PVPolicies *PVCleanupPolicy `json:"pvPolicies,omitempty"`
}
```
- `omitempty`
	- 값이 없을 때는 JSON 출력에서 생략.
	- `omitempty`는 Go 구조체 필드의 값이 해당 타입의 기본값(Zero Value)일 경우, JSON으로 변환할 때 해당 필드를 결과 JSON 문자열에서 완전히 생략(Omit)하도록 지시하는 태그이다.
		- `int, int32, int64` 등 정수 -> 기본값 : 0
		- `bool` -> 기본값 : false
		- `string` -> 기본값 : 빈 문자열 (`""`)
		- `slice, map, interface, pointer` -> 기본값 : `nil`
		- `struct`  -> 기본값 : 모든 필드가 기본값인 상태 (복합적 판단 필요)
- `[]string`
	- `[]` : 하나의 값이 아닌, 여러 개의 값을 담을 수 았는 List 형태의 데이터라는 의미이다.

```go
// PodCleanupPolicy defines cleanup rules for Pods
type PodCleanupPolicy struct {
	// Enable pod cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Clean up failed pods (CrashLoopBackOff, Error, etc.)
	// +optional
	FailedPods *FailedPodPolicy `json:"failedPods,omitempty"`

	// Clean up idle pods (low resource usage)
	// +optional
	IdlePods *IdlePodPolicy `json:"idlePods,omitempty"`
}
```
- `*FailedPodPolicy`
	- `*` 기호는 Go 언어에서 참조 기능을 하는 포인터(Pointer)를 의미한다.


```go
// FailedPodPolicy defines rules for failed pod cleanup
type FailedPodPolicy struct {
	// Enable failed pod cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// States to clean up
	// +kubebuilder:default={"CrashLoopBackOff","ImagePullBackOff","Pending","Error","Evicted","Failed"}
	States []string `json:"states,omitempty"`

	// Minimum age before cleanup (duration format: 1h, 24h, 7d)
	// +kubebuilder:default="3h"
	MinAge string `json:"minAge,omitempty"`
}

// IdlePodPolicy defines rules for idle pod cleanup
type IdlePodPolicy struct {
	// Enable idle pod cleanup
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// CPU usage threshold percentage (e.g., 10 means < 10%)
	// +kubebuilder:default=10
	CPUThresholdPercent int `json:"cpuThresholdPercent,omitempty"`

	// Memory usage threshold percentage
	// +kubebuilder:default=15
	MemoryThresholdPercent int `json:"memoryThresholdPercent,omitempty"`

	// Duration pod must be idle before cleanup (e.g., "7d")
	// +kubebuilder:default="7d"
	IdleDuration string `json:"idleDuration,omitempty"`
}

// PVCleanupPolicy defines cleanup rules for PersistentVolumes
type PVCleanupPolicy struct {
	// Enable PV cleanup
	// +kubebuilder:default=true
	Enabled bool `json:"enabled,omitempty"`

	// Minimum age for unused PVs (e.g., "30d")
	// +kubebuilder:default="30d"
	MinAge string `json:"minAge,omitempty"`

	// Only clean PVs in these states
	// +kubebuilder:default={"Released","Available"}
	States []string `json:"states,omitempty"`
}

// CleanupPolicyStatus defines the observed state of CleanupPolicy
type CleanupPolicyStatus struct {
	// Last execution time
	LastExecutionTime *metav1.Time `json:"lastExecutionTime,omitempty"`

	// Next scheduled execution time
	NextExecutionTime *metav1.Time `json:"nextExecutionTime,omitempty"`

	// Total resources marked for cleanup
	MarkedForCleanup int `json:"markedForCleanup,omitempty"`

	// Total resources cleaned up
	CleanedUp int `json:"cleanedUp,omitempty"`

	// Conditions represent the latest available observations of an object's state
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Resources pending approval
	// +optional
	PendingApproval []PendingResource `json:"pendingApproval,omitempty"`
}
```
- `metav1`
	- Go 언어에서 쿠버네티스 메타데이터 관련 타입들이 정의된 패키지를 의미한다. 이는 쿠버네티스 리소스의 모든 스펙에 공통적으로 사용되는 기본적인 정의들을 포함한다.
	- `metav1`은 Go 언어의 `k8s.io/apimachinery/pkg/apis/meta/v1` 경로에 정의된 패키지를 짧게 부르는 Alias 이다.
	- 쿠버네티스 리소스라면 반드시 포함하는 핵심적인 타입들이 들어있다.
		- `ObjectMeta` : 모든 쿠버네티스 객체의 **이름, 네임스페이스, 라벨, 어노테이션** 등 메타데이터를 정의하는 구조체.
		- `TypeMeta` : 객체의 종류(`Kind`)와 API 버전(`APIVersion`)을 정의하는 구조체.
		- `Time` : 시간을 나타내는 타입.
		- ==`Conditions`== : 리소스의 현재 **상태 정보**를 표준화된 방식으로 나타내는 타입.
			- ==쿠버네티스는 파드(Pod)나 디플로이먼트(Deployment)와 같은 모든 내장 리소스의 상태를 보고할 때 이 `Conditions` 타입을 사용한다.==
				- 예를 들어, 파드의 상태가 `Ready`인지 아닌지, 혹은 PVC가 성공적으로 바인딩되었는지 등을 `Conditions`의 배열로 표현한다.
				- 따라서, 오브젝트의 상태를 파악하는데 필수적으로 확인해야 하는 타입이다.

- `metav1.Conditions`의 주요 필드

|**필드**|**타입**|**설명**|**예시**|
|---|---|---|---|
|**`Type`**|`string`|상태의 종류|`"PolicyActive"`, `"CleanupFailed"`|
|**`Status`**|`ConditionStatus`|상태의 값 (`True`, `False`, `Unknown`)|`"True"`|
|**`Reason`**|`string`|상태가 된 이유를 짧게 명시|`"SuccessfulReconciliation"`|
|**`Message`**|`string`|상태에 대한 자세한 설명|`"All defined cleanup policies ran successfully."`|


```go
// PendingResource represents a resource waiting for approval
type PendingResource struct {
	// Resource type (Pod, PersistentVolume, etc.)
	Kind string `json:"kind"`

	// Resource namespace
	Namespace string `json:"namespace,omitempty"`

	// Resource name
	Name string `json:"name"`

	// Reason for cleanup
	Reason string `json:"reason"`

	// Time marked for cleanup
	MarkedAt metav1.Time `json:"markedAt"`
}
```

```go
//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Cluster
//+kubebuilder:printcolumn:name="DryRun",type="boolean",JSONPath=".spec.dryRun"
//+kubebuilder:printcolumn:name="Last Execution",type="date",JSONPath=".status.lastExecutionTime"
//+kubebuilder:printcolumn:name="Cleaned Up",type="integer",JSONPath=".status.cleanedUp"
```
- 주석처럼 보이지만, `kubebuilder`에 의해서 실제로 동작하는 지시어이다. kubebiolder Marker 라고 불리며, kubebuilder가 이 주석들을 읽어 CRD 파일과 Controller 코드를 자동으로 생성 및 갱신하는데 사용 한다.
- kubebuilder 마커는 바로 다음에 오는 Go 구조체에 대해서만 적용된다.
- #### 1) 기본 API 객체 정의
	- `//+kubebuilder:object:root=true`
		- 다음 구조체가 Root 레벨의 쿠버네티스 API 객체임을 선언한다. 이 마거가 없으면 CRD 파일이 생성되지 않는다.
	- `//+kubebuilder:subresource:status`
		- 리소스에 `/status` 서브리소스를 활성화한다. 이를 통해 `Status` 필드만 별도로 업데이트 가능하며, 컨트롤러가 `Spec` 필드에 영향을 주지 않고, `Status` 필드만 업데이트할 수 있다.
- #### 2) 리소스 범위 및 속성 정의
	- `//+kubebuilder:resource:scope=Cluster`
		- 이 구조체(`CleanupPolicy`) CRD가 클러스터 전체를 스코프로 하여 동작하도록 정의한다.
- #### 3) `kubectl get` 출력 설정 (AdditionalPrinterColumns)
- `//+kubebuilder:printcolumn:name="DryRun",type="boolean",JSONPath=".spec.dryRun"`
	- `//+kubebuilder:printcolumn:name="Last Execution",type="date",JSONPath=".status.lastExecutionTime"
	- `//+kubebuilder:printcolumn:name="CleanedUp",type="integer",JSONPath=".status.cleanedUp"`
	- 사용자가 `kubectl get` 명령을 실행했을 때, 기본 정보(`NAME, AGE`) 외 추가로적으로 보고싶은 정보를 컬럼으로 표시하도록 CRD에 정의하는 부분이다.

```go
// CleanupPolicy is the Schema for the cleanuppolicies API
type CleanupPolicy struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   CleanupPolicySpec   `json:"spec,omitempty"`
	Status CleanupPolicyStatus `json:"status,omitempty"`
}
```
- `metav1.TypeMeta` : 이 객체가 어떤 `Kind`이며, 어떤 API 그룹/버전에 속하는 지를 정의한다.
- `json:",inline"` : 이 `TypeMeta`에 속하는 내부 필드들을 JSON 출력 시 최상단에 출력하도록 한다.
- `metav1.ObjectMeta` : 모든 쿠버네티스 객체의 기본정보를 담는 표준 필드이다.
- `json:"metadata,omitempty"` : 해당 필드의 JSON 키 이름이 `metadata`가 되도록 지정한다.
- `Spec CleanupPolicySpec` : 위에서 정의한 Cleanup 정책 규칙이 포함된 구조체를 의미한다.
- `Status CleanupPolicyStatus` : 위에서 정의한 오퍼레이터가 기록할 결과 정보가 포함된 구조체를 의미한다.

요약 및 정리

|**구분**|**YAML 파일 구조**|**Go 구조체 필드**|
|---|---|---|
|**시스템 정보**|`apiVersion`, `kind`|`metav1.TypeMeta`|
|**관리 정보**|`metadata`|`metav1.ObjectMeta`|
|**사용자 요청**|`spec`|`CleanupPolicySpec`|
|**오퍼레이터 결과**|`status`|`CleanupPolicyStatus`|

```go
//+kubebuilder:object:root=true

// CleanupPolicyList contains a list of CleanupPolicy
type CleanupPolicyList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []CleanupPolicy `json:"items"`
}
```
- `Items           []CleanupPolicy `json:"items"``
	- 이 슬라이스가 `CleanupPolicy` 객체들을 담고 있다. `kubectl get` 명령으로 조회되는 핵심 데이터이다.

```go
func init() {
	SchemeBuilder.Register(&CleanupPolicy{}, &CleanupPolicyList{})
}
```
- `func init()`
	- Go 언어의 `init` 함수는 패키지가 로드될 때 자동으로 한번 실행된다.
- ==`SchemeBuilder.Register(...)`==
	- 쿠버네티스 컨트롤러 런타임이 사용하는 `Scheme` 레지스트리에 새로운 CRD를 등록하는 역할을 한다.
	- 오퍼레이터가 위에서 정의한 새로운 사용자 정의 리소스 타입들(`CleanupPolicy, CleanupPolicyList`)을 쿠버네티스의 다른 내장 리소스처럼 인식하고 처리할 수 있도록 시스템에 알린다.


## 2. `make generate && manifests && install`
### 2.1 `make generate`
```bash
$ make generate
/Users/been/beengineer/300_Cloud_Club/CC8th_Study2nd_K8s_Operator/08th-k8s-operator/monitoring-been/bin/controller-gen object:headerFile="hack/boilerplate.go.txt" paths="./..."
```
- `make generate`를 수행하면 변경사항을 반영해서, DeepCopy, DeepCopyInto, DeepCopyObject 메서드를 생성한다.

### 2.2 `make manifests`
```bash
$ make manifests
/Users/been/beengineer/300_Cloud_Club/CC8th_Study2nd_K8s_Operator/08th-k8s-operator/monitoring-been/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
```
- `make manifests` 를 수행하면,CRD 매니페스트를 생성한다.
- 매니페스트는 `config/crd/bases`에 생성된다.

```yaml
$ cat config/crd/bases/cleanup.cloudclub.com_cleanuppolicies.yaml 
---
apiVersion: apiextensions.k8s.io/v1
kind: CustomResourceDefinition
metadata:
  annotations:
    controller-gen.kubebuilder.io/version: v0.18.0
  name: cleanuppolicies.cleanup.cloudclub.com
spec:
  group: cleanup.cloudclub.com
  names:
    kind: CleanupPolicy
    listKind: CleanupPolicyList
    plural: cleanuppolicies
    singular: cleanuppolicy
  scope: Cluster
  versions:
  - additionalPrinterColumns:
    - jsonPath: .spec.dryRun
      name: DryRun
      type: boolean
    - jsonPath: .status.lastExecutionTime
      name: Last Execution
      type: date
    - jsonPath: .status.cleanedUp
      name: Cleaned Up
      type: integer
    name: v1alpha1
    schema:
      openAPIV3Schema:
        description: CleanupPolicy is the Schema for the cleanuppolicies API.
        properties:
          apiVersion:
            description: |-
              APIVersion defines the versioned schema of this representation of an object.
              Servers should convert recognized schemas to the latest internal value, and
              may reject unrecognized values.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#resources
            type: string
          kind:
            description: |-
              Kind is a string value representing the REST resource this object represents.
              Servers may infer this from the endpoint the client submits requests to.
              Cannot be updated.
              In CamelCase.
              More info: https://git.k8s.io/community/contributors/devel/sig-architecture/api-conventions.md#types-kinds
            type: string
          metadata:
            type: object
          spec:
            description: CleanupPolicySpec defines the desired state of CleanupPolicy.
            properties:
              dryRun:
                default: false
                description: DryRun mode - if true, only log actions woithout actual
                  deletion
                type: boolean
              excludeNamespaces:
                default:
                - kube-system
                - kube-public
                - kube-node-lease
                description: Namespaces to exclude from cleanup
                items:
                  type: string
                type: array
              includeNamespaces:
                description: Namespaces to include (empty means all namespaces)
                items:
                  type: string
                type: array
              podPolicies:
                description: Pod cleanup policies
                properties:
                  enabled:
                    default: true
                    description: Enable pod cleanup
                    type: boolean
                  failedPods:
                    description: Clean up failed pods (CrashLoopBackOff etc.)
                    properties:
                      enabled:
                        default: true
                        description: Enable failed pod cleanup
                        type: boolean
                      minAge:
                        default: 3h
                        description: 'Minimum age before cleanup (duration format:
                          1h, 24h, 7d)'
                        type: string
                      states:
                        default:
                        - CrashLoopBackOff
                        - ImagePullBackOff
                        - Pending
                        - Error
                        - Evicted
                        - Failed
                        description: Status to clean up
                        items:
                          type: string
                        type: array
                    type: object
                  idelPods:
                    description: Clean up idle pods (low resource usage)
                    properties:
                      cpuTresholdPercent:
                        description: CPU Usage threshold percentage (e.g. 10 means
                          < 10%)
                        type: integer
                      enabled:
                        default: true
                        description: Enable idle pod cleanup
                        type: boolean
                      idleDuration:
                        default: 14d
                        description: Dration pod must be idle before cleanup (e.g.
                          14d)
                        type: string
                      memoryTresholdPercent:
                        default: 15
                        description: Memory Usage treshold percentage
                        type: integer
                    type: object
                type: object
              pvPolicies:
                description: PersistentVolume cleanup policies
                properties:
                  minAge:
                    default: 14d
                    description: Minimum age for unused PVs (e.g. "14d")
                    type: string
                  states:
                    description: Only clean PVs in these states
                    items:
                      type: string
                    type: array
                  "true":
                    default: true
                    description: Enable PV cleanup
                    type: boolean
                type: object
              requireApproval:
                default: false
                description: RequireApproval - if true, mart resources for deletion
                  but wait for manual approval
                type: boolean
              schedule:
                default: 0 6 * * *
                type: string
            type: object
          status:
            description: CleanupPolicyStatus defines the observed state of CleanupPolicy
            properties:
              cleanedUp:
                description: Total resources cleaned up
                type: integer
              conditions:
                description: Conditions represet the latest available observations
                  of an object's state
                items:
                  description: Condition contains details for one aspect of the current
                    state of this API Resource.
                  properties:
                    lastTransitionTime:
                      description: |-
                        lastTransitionTime is the last time the condition transitioned from one status to another.
                        This should be when the underlying condition changed.  If that is not known, then using the time when the API field changed is acceptable.
                      format: date-time
                      type: string
                    message:
                      description: |-
                        message is a human readable message indicating details about the transition.
                        This may be an empty string.
                      maxLength: 32768
                      type: string
                    observedGeneration:
                      description: |-
                        observedGeneration represents the .metadata.generation that the condition was set based upon.
                        For instance, if .metadata.generation is currently 12, but the .status.conditions[x].observedGeneration is 9, the condition is out of date
                        with respect to the current state of the instance.
                      format: int64
                      minimum: 0
                      type: integer
                    reason:
                      description: |-
                        reason contains a programmatic identifier indicating the reason for the condition's last transition.
                        Producers of specific condition types may define expected values and meanings for this field,
                        and whether the values are considered a guaranteed API.
                        The value should be a CamelCase string.
                        This field may not be empty.
                      maxLength: 1024
                      minLength: 1
                      pattern: ^[A-Za-z]([A-Za-z0-9_,:]*[A-Za-z0-9_])?$
                      type: string
                    status:
                      description: status of the condition, one of True, False, Unknown.
                      enum:
                      - "True"
                      - "False"
                      - Unknown
                      type: string
                    type:
                      description: type of condition in CamelCase or in foo.example.com/CamelCase.
                      maxLength: 316
                      pattern: ^([a-z0-9]([-a-z0-9]*[a-z0-9])?(\.[a-z0-9]([-a-z0-9]*[a-z0-9])?)*/)?(([A-Za-z0-9][-A-Za-z0-9_.]*)?[A-Za-z0-9])$
                      type: string
                  required:
                  - lastTransitionTime
                  - message
                  - reason
                  - status
                  - type
                  type: object
                type: array
              lastExecutionTime:
                description: Last execution time
                format: date-time
                type: string
              nextExecutionTome:
                description: Next scheduled execution time
                format: date-time
                type: string
              pendingApproval:
                description: Resources pending approval
                items:
                  description: PendingResource represents a resource waiting for approval
                  properties:
                    kind:
                      description: Resource type (Pod, PV, etc.)
                      type: string
                    markedAt:
                      description: Time marked for cleanup
                      format: date-time
                      type: string
                    name:
                      description: Resource name
                      type: string
                    namespace:
                      description: Resource namespace
                      type: string
                    reason:
                      description: Reason for cleanup
                      type: string
                  required:
                  - kind
                  - markedAt
                  - name
                  - reason
                  type: object
                type: array
            type: object
        type: object
    served: true
    storage: true
    subresources:
      status: {}
```

### 2.3 `make install`
```bash
$ make install
/Users/been/beengineer/300_Cloud_Club/CC8th_Study2nd_K8s_Operator/08th-k8s-operator/monitoring-been/bin/controller-gen rbac:roleName=manager-role crd webhook paths="./..." output:crd:artifacts:config=config/crd/bases
Downloading sigs.k8s.io/kustomize/kustomize/v5@v5.6.0
go: downloading sigs.k8s.io/kustomize/kustomize/v5 v5.6.0
go: downloading sigs.k8s.io/kustomize/cmd/config v0.19.0
go: downloading github.com/spf13/cobra v1.8.0
go: downloading golang.org/x/text v0.21.0
go: downloading github.com/sergi/go-diff v1.2.0
go: downloading k8s.io/kube-openapi v0.0.0-20241212222426-2c72e554b1e7
go: downloading google.golang.org/protobuf v1.35.1
/Users/been/beengineer/300_Cloud_Club/CC8th_Study2nd_K8s_Operator/08th-k8s-operator/monitoring-been/bin/kustomize build config/crd | kubectl apply -f -
customresourcedefinition.apiextensions.k8s.io/cleanuppolicies.cleanup.cloudclub.com created
```

### 2.4 확인
```bash
$ kubectl get crds
NAME                                             CREATED AT
...
cleanuppolicies.cleanup.cloudclub.com            2025-11-25T21:31:09Z
...
```

### 2.5 삭제
```bash
$ make uninstall

# 또는 

$ kubectl delete -f ./config/crd/bases
```
