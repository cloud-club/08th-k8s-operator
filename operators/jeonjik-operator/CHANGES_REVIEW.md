# Jeonjik Operator 변경사항 리뷰

## 개요

이 문서는 Jeonjik Operator에 KubeVirt VirtualMachine 생성 기능을 추가한 변경사항에 대한 상세 리뷰입니다.

**변경 목적**: Jeonjik CR의 specType에 따라 자동으로 KubeVirt VirtualMachine 리소스를 생성하여 가상머신을 프로비저닝하는 기능 추가

**변경 일자**: 2025년

**중요 설계 원칙**: 
- VirtualMachine은 **생성만** 하고, 이후 업데이트는 하지 않음
- VirtualMachine의 생명주기는 **KubeVirt 컨트롤러**가 관리
- Jeonjik CR 삭제 시 VirtualMachine은 **Owner Reference**를 통해 자동 삭제

---

## 주요 변경사항 요약

1. **의존성 추가**: `kubevirt.io/api` 패키지 추가
2. **Scheme 등록**: main.go에 KubeVirt API scheme 등록
3. **Controller 로직 확장**: VirtualMachine 생성 로직 추가 (업데이트 제외)
4. **RBAC 권한 추가**: VirtualMachine 리소스 접근 권한 추가
5. **버그 수정**: specTypes 맵의 문법 오류 수정
6. **Deprecation 해결**: `spec.running` → `spec.runStrategy` 변경

---

## 1. 의존성 추가

### 변경 파일
- `go.mod`
- `go.sum`

### 변경 내용

```go
require (
    // ... 기존 의존성들 ...
    kubevirt.io/api v1.7.0  // 추가됨
)
```

### 상세 설명

- **패키지 선택**: `kubevirt.io/client-go` 대신 `kubevirt.io/api`를 선택
  - 이유: `client-go`는 의존성 충돌 발생 (k8s.io/kube-openapi 버전 불일치)
  - `api` 패키지는 타입 정의만 포함하여 가볍고 충돌이 적음
  - controller-runtime의 client.Client를 통해 리소스를 조작하므로 타입만 있으면 충분

- **버전**: v1.7.0 (최신 안정 버전)

### 추가된 간접 의존성
- `kubevirt.io/containerized-data-importer-api`
- `kubevirt.io/controller-lifecycle-operator-sdk/api`

---

## 2. main.go 변경

### 변경 파일
- `cmd/main.go`

### 변경 내용

#### 2.1 Import 추가

```go
import (
    // ... 기존 imports ...
    kubevirtv1 "kubevirt.io/api/core/v1"  // 추가됨
)
```

#### 2.2 Scheme 등록

```go
func init() {
    utilruntime.Must(clientgoscheme.AddToScheme(scheme))
    utilruntime.Must(ccpcroomkrv1.AddToScheme(scheme))
    utilruntime.Must(kubevirtv1.AddToScheme(scheme))  // 추가됨
}
```

### 상세 설명

**왜 scheme 등록이 필요한가?**

1. **타입 인식**: controller-runtime이 KubeVirt VirtualMachine 리소스를 Go 타입으로 인식
2. **직렬화/역직렬화**: Kubernetes API Server와 통신 시 JSON/YAML 변환
3. **타입 안정성**: 컴파일 타임에 타입 체크 가능

**동작 원리**:
- Manager 생성 시 scheme이 사용됨
- scheme에 등록된 타입만 controller-runtime이 처리 가능
- 등록하지 않으면 VirtualMachine 리소스를 생성할 수 없음

---

## 3. Controller 로직 확장

### 변경 파일
- `internal/controller/jeonjik_controller.go`

### 3.1 Import 추가

```go
import (
    "context"
    "fmt"           // 추가: 문자열 포맷팅
    "strconv"       // 추가: 문자열-숫자 변환
    "strings"       // 추가: 문자열 처리

    corev1 "k8s.io/api/core/v1"                    // 추가: 리소스 제한 설정
    "k8s.io/apimachinery/pkg/api/resource"         // 추가: 리소스 양 표현
    metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"  // 추가: 메타데이터
    // ... 기존 imports ...
    kubevirtv1 "kubevirt.io/api/core/v1"           // 추가: KubeVirt 타입
)
```

### 3.2 RBAC 주석 추가

```go
// +kubebuilder:rbac:groups=kubevirt.io,resources=virtualmachines,verbs=get;list;watch;create;delete
```

**설명**:
- kubebuilder 주석으로 RBAC 권한 자동 생성
- VirtualMachine 리소스에 대한 조회 및 생성 권한만 필요
- **주의**: `update`, `patch` 권한은 제거됨 (업데이트하지 않으므로)
- `delete` 권한은 Owner Reference로 인한 자동 삭제 시 필요
- `make manifests` 실행 시 자동으로 `config/rbac/role.yaml`에 반영됨

### 3.3 Reconcile 함수 확장

#### 변경 전
```go
func (r *JeonjikReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ... status 업데이트만 수행 ...
    return ctrl.Result{}, nil
}
```

#### 변경 후
```go
func (r *JeonjikReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
    // ... status 업데이트 ...
    
    // VirtualMachine 생성 로직 (존재하지 않을 때만)
    vmNamespace := "kubevirt-instance"
    vmName := cr.Name
    
    var vm kubevirtv1.VirtualMachine
    err := r.Get(ctx, client.ObjectKey{Namespace: vmNamespace, Name: vmName}, &vm)
    if err != nil {
        if client.IgnoreNotFound(err) != nil {
            // 에러 처리
        }
        // VirtualMachine doesn't exist, create it
        vm = r.buildVirtualMachine(cr, spec, vmName, vmNamespace)
        r.Create(ctx, &vm)
    } else {
        // VirtualMachine already exists, do nothing (KubeVirt controller manages it)
    }
    
    return ctrl.Result{}, nil
}
```

**주요 변경점**:
1. **VirtualMachine 조회**: 기존 VM이 있는지 확인
2. **조건부 생성**: 존재하지 않을 때만 생성
3. **업데이트 제외**: 존재하면 아무 작업도 하지 않음 (KubeVirt 컨트롤러가 관리)

**설계 원칙**:
- 우리는 VirtualMachine을 **생성만** 함
- 이후 VirtualMachine의 생명주기는 **KubeVirt 컨트롤러**가 관리
- 충돌 방지: KubeVirt 컨트롤러와의 충돌을 방지하기 위해 업데이트하지 않음

### 3.4 buildVirtualMachine 함수 추가

**목적**: Jeonjik CR의 specType 정보를 바탕으로 VirtualMachine 리소스 생성

**주요 로직**:

1. **CPU 파싱**
   ```go
   cpuStr := strings.TrimSuffix(spec.CPU, "c")  // "2c" -> "2"
   cpuCores, _ := strconv.ParseInt(cpuStr, 10, 32)
   ```

2. **Memory 파싱**
   ```go
   memoryStr := strings.TrimSuffix(spec.Memory, "m")  // "2m" -> "2"
   memoryGi, _ := strconv.ParseInt(memoryStr, 10, 32)
   memoryQuantity := resource.MustParse(fmt.Sprintf("%dGi", memoryGi))
   ```
   - **주의**: 현재 "m"을 Gi로 해석하고 있음 (실제 요구사항 확인 필요)

3. **RunStrategy 설정** (Deprecation 해결)
   ```go
   RunStrategy: func() *kubevirtv1.VirtualMachineRunStrategy {
       strategy := kubevirtv1.RunStrategyAlways
       return &strategy
   }(),
   ```
   - **변경 사항**: `spec.running` (deprecated) → `spec.runStrategy` 사용
   - `RunStrategyAlways`: VM이 항상 실행되도록 설정 (기존 `running: true`와 동일)

4. **Owner Reference 설정**
   ```go
   OwnerReferences: []metav1.OwnerReference{
       {
           APIVersion: cr.APIVersion,
           Kind:       cr.Kind,
           Name:       cr.Name,
           UID:        cr.UID,
           Controller: func() *bool { b := true; return &b }(),
       },
   }
   ```
   - Jeonjik CR 삭제 시 VirtualMachine도 자동 삭제됨 (Cascading Delete)
   - **Finalizer 불필요**: Owner Reference만으로 충분

5. **GPU 지원**
   ```go
   if spec.GPU != "0" {
       gpuCount, _ := strconv.ParseInt(strings.TrimSuffix(spec.GPU, "gpu"), 10, 32)
       if gpuCount > 0 {
           vm.Spec.Template.Spec.Domain.Devices.GPUs = []kubevirtv1.GPU{...}
           vm.Spec.Template.Spec.Domain.Resources.Limits["nvidia.com/gpu"] = ...
       }
   }
   ```

**개선 필요 사항**:
- OS 이미지 매핑: 현재 모든 OS에 대해 동일한 기본 이미지 사용
  ```go
  osImages := map[string]string{
      "ubuntu-24": "quay.io/kubevirt/fedora-cloud-container-disk-demo:latest",
      "rocky-9":   "quay.io/kubevirt/fedora-cloud-container-disk-demo:latest",
  }
  ```
  → 실제 OS별 이미지로 교체 필요

### 3.5 getOSImage 함수 추가

**목적**: OS 타입에 따른 컨테이너 이미지 반환

**현재 상태**: 모든 OS에 대해 동일한 기본 이미지 반환 (임시)

**개선 필요**: 실제 OS별 이미지 매핑 필요

### 3.6 CreateOrUpdateVirtualMachine 함수 제거

**변경 사항**: 
- 이전에는 VirtualMachine을 생성하고 업데이트하는 함수가 있었음
- 현재는 **생성만** 하므로 함수 제거
- Reconcile 함수에서 직접 `r.Create()` 호출

**이유**:
- VirtualMachine은 KubeVirt 컨트롤러가 관리
- 우리가 업데이트하면 충돌 발생 가능
- 생성만 하고 이후는 관여하지 않음

---

## 4. RBAC 권한 추가

### 변경 파일
- `config/rbac/role.yaml`

### 변경 내용

```yaml
- apiGroups:
  - kubevirt.io
  resources:
  - virtualmachines
  verbs:
  - create
  - delete
  - get
  - list
  - watch
```

### 상세 설명

**필요한 권한**:
- `get`, `list`, `watch`: VirtualMachine 리소스 조회 및 감시 (존재 여부 확인)
- `create`: 새로운 VirtualMachine 생성
- `delete`: VirtualMachine 삭제 (Owner Reference로 인한 자동 삭제 시 필요)

**제거된 권한**:
- `update`, `patch`: VirtualMachine을 업데이트하지 않으므로 불필요

**권한 범위**:
- ClusterRole이므로 클러스터 전체의 VirtualMachine에 접근 가능
- 네임스페이스 제한이 필요하면 Role + RoleBinding으로 변경 고려

---

## 5. 버그 수정

### 변경 파일
- `internal/controller/jeonjik_controller.go`

### 5.1 specTypes 맵 문법 오류 수정

#### 변경 전 (문법 오류)
```go
specTypes := map[string]struct {
    CPU    string
    Memory string
    OS     string
    GPU    string
}{
    "slave-jiwon":  {"2c", "2m", "ubuntu-24", "0"},
    "seoksa-jiwon": {"3c", "3m", "ubuntu-24", "0"},
    "baksa-jiwon":  {"4c", "4m", "ubuntu-24", "0"},
    "h100-jjaegii": {"4c", "4m", "rocky-9", "4gpu"}, "l40s-jjaegii": {"8c", "8m", "rocky-9", "8gpu"},  // ❌ 문법 오류
    "moon0-potato": {"4c", "4m", "ubuntu-24", "0"},
}
```

#### 변경 후
```go
specTypes := map[string]struct {
    CPU    string
    Memory string
    OS     string
    GPU    string
}{
    "slave-jiwon":  {"2c", "2m", "ubuntu-24", "0"},
    "seoksa-jiwon": {"3c", "3m", "ubuntu-24", "0"},
    "baksa-jiwon":  {"4c", "4m", "ubuntu-24", "0"},
    "h100-jjaegii": {"4c", "4m", "rocky-9", "4gpu"},   // ✅ 수정됨
    "l40s-jjaegii": {"8c", "8m", "rocky-9", "8gpu"},  // ✅ 별도 라인으로 분리
    "moon0-potato": {"4c", "4m", "ubuntu-24", "0"},
}
```

**문제점**: 두 개의 map 항목이 한 줄에 있어서 Go 컴파일러가 파싱하지 못함

### 5.2 Deprecation 경고 해결

#### 변경 전
```go
Spec: kubevirtv1.VirtualMachineSpec{
    Running: func() *bool { b := true; return &b }(),
    ...
}
```

#### 변경 후
```go
Spec: kubevirtv1.VirtualMachineSpec{
    RunStrategy: func() *kubevirtv1.VirtualMachineRunStrategy {
        strategy := kubevirtv1.RunStrategyAlways
        return &strategy
    }(),
    ...
}
```

**변경 이유**:
- `spec.running` 필드가 deprecated됨
- KubeVirt v1.7.0에서 `spec.runStrategy` 사용 권장
- `RunStrategyAlways`: VM이 항상 실행되도록 설정 (기존 `running: true`와 동일)

---

## 아키텍처 변경

### 변경 전
```
Jeonjik CR
    ↓
Reconcile
    ↓
Status 업데이트만 수행
```

### 변경 후
```
Jeonjik CR 생성/수정
    ↓
Reconcile 호출
    ↓
┌─────────────────┐
│ Status 업데이트 │
└─────────────────┘
    ↓
┌──────────────────────────┐
│ VirtualMachine 존재 확인 │
└───────────┬──────────────┘
            │
    ┌───────┴────────┐
    │                │
존재하지 않음      존재함
    │                │
    ↓                ↓
┌──────────┐    ┌──────────┐
│ 생성     │    │ 아무것도 │
│          │    │ 안 함    │
└────┬─────┘    └────┬─────┘
     │               │
     └───────┬───────┘
             ↓
     KubeVirt Controller (기존)
             ↓
         실제 VM 생성/관리
```

### 역할 분담

1. **Jeonjik Operator**: 
   - Jeonjik CR → VirtualMachine CR 변환
   - specType에 따른 리소스 스펙 매핑
   - **VirtualMachine 생성만** 수행
   - 이후 VirtualMachine 관리는 하지 않음

2. **KubeVirt Controller** (기존):
   - VirtualMachine CR → 실제 VM 생성/관리
   - VM 라이프사이클 관리
   - VirtualMachine 스펙 변경 감지 및 반영

### 삭제 흐름

```
Jeonjik CR 삭제 요청
    ↓
Owner Reference에 의해
    ↓
VirtualMachine 자동 삭제 (Kubernetes가 처리)
    ↓
KubeVirt Controller가 VM 종료 처리
```

**중요**: Finalizer를 사용하지 않음. Owner Reference만으로 충분하며, Kubernetes가 자동으로 Cascading Delete를 수행함.

---

## 테스트 방법

### 1. 로컬 개발 환경

```bash
# 1. CRD 설치
make install

# 2. kubevirt-instance 네임스페이스 생성
kubectl create namespace kubevirt-instance

# 3. 오퍼레이터 실행
make run

# 4. 다른 터미널에서 테스트
kubectl apply -f config/samples/v2_jeonjik.yaml

# 5. 확인
kubectl get jeonjik
kubectl get vm -n kubevirt-instance

# 6. VirtualMachine이 이미 존재하는 경우 재시도
# (아무것도 하지 않아야 함)
kubectl delete jeonjik jeonjik-sample
kubectl apply -f config/samples/v2_jeonjik.yaml

# 7. 삭제 테스트 (Owner Reference 동작 확인)
kubectl delete jeonjik jeonjik-sample
kubectl get vm -n kubevirt-instance  # 자동 삭제 확인
```

### 2. 클러스터 배포

```bash
# 1. 이미지 빌드 및 푸시
export IMG=your-registry/jeonjik-operator:latest
make docker-build IMG=$IMG
make docker-push IMG=$IMG

# 2. CRD 설치
make install

# 3. 오퍼레이터 배포
make deploy IMG=$IMG

# 4. 테스트
kubectl apply -f config/samples/v2_jeonjik.yaml
```

---

## 알려진 이슈 및 개선 사항

### 1. OS 이미지 매핑
- **현재**: 모든 OS에 대해 동일한 기본 이미지 사용
- **개선**: 실제 OS별 이미지로 교체 필요
  ```go
  osImages := map[string]string{
      "ubuntu-24": "quay.io/kubevirt/ubuntu-24:latest",
      "rocky-9":   "quay.io/kubevirt/rocky-9:latest",
  }
  ```

### 2. Memory 단위 해석
- **현재**: "2m"을 2Gi로 해석
- **확인 필요**: 실제 요구사항이 "m"이 Gi를 의미하는지 확인

### 3. 에러 처리
- **현재**: 일부 파싱 오류 시 기본값 사용 (strconv.ParseInt의 두 번째 반환값 무시)
- **개선**: 파싱 실패 시 명시적 에러 처리 고려

### 4. 네임스페이스 하드코딩
- **현재**: `vmNamespace := "kubevirt-instance"` 하드코딩
- **개선**: ConfigMap 또는 CRD spec으로 설정 가능하도록 변경 고려

### 5. VirtualMachine 업데이트 전략
- **현재**: 생성만 하고 업데이트하지 않음 ✅
- **설계 의도**: KubeVirt 컨트롤러가 관리하므로 우리는 관여하지 않음

---

## 보안 고려사항

1. **RBAC 권한**: ClusterRole이므로 클러스터 전체 VirtualMachine 접근 가능
   - 필요시 Role + RoleBinding으로 범위 제한 고려
   - 현재는 `update`, `patch` 권한 제거로 보안 강화

2. **이미지 소스**: 컨테이너 이미지의 신뢰성 확인 필요

3. **리소스 제한**: VirtualMachine 생성 시 리소스 제한 설정 고려

---

## 성능 고려사항

1. **Reconcile 빈도**: 현재 변경사항이 없어도 Reconcile 호출 가능
   - controller-runtime이 자동으로 최적화
   - VirtualMachine이 이미 존재하면 빠르게 스킵

2. **Watch 효율성**: Kubernetes Watch API 사용으로 효율적

3. **Owner Reference**: Cascading Delete로 인한 부하 고려
   - 하지만 Finalizer를 사용하지 않으므로 오버헤드 최소화

4. **충돌 방지**: VirtualMachine을 업데이트하지 않으므로 KubeVirt 컨트롤러와 충돌 없음

---

## 설계 결정 사항

### 1. VirtualMachine 업데이트를 하지 않는 이유

**문제 상황**:
- KubeVirt 컨트롤러가 VirtualMachine을 관리함
- 우리가 VirtualMachine을 업데이트하면 충돌 발생 가능
- ResourceVersion 충돌 오류 발생

**해결 방법**:
- VirtualMachine은 생성만 하고 이후 업데이트하지 않음
- KubeVirt 컨트롤러가 VirtualMachine의 생명주기 관리
- 명확한 책임 분리

### 2. Finalizer를 사용하지 않는 이유

**Owner Reference만으로 충분**:
- Owner Reference에 `Controller: true` 설정
- Kubernetes가 자동으로 Cascading Delete 수행
- Finalizer는 불필요한 복잡도만 추가

**장점**:
- 코드 단순화
- 성능 향상 (Finalizer 처리 오버헤드 없음)
- 표준 Kubernetes 패턴 사용

---

## 참고 자료

- [KubeVirt API Documentation](https://kubevirt.io/api/)
- [controller-runtime Documentation](https://pkg.go.dev/sigs.k8s.io/controller-runtime)
- [Kubernetes Owner References](https://kubernetes.io/docs/concepts/overview/working-with-objects/owners-dependents/)
- [KubeVirt RunStrategy](https://kubevirt.io/api/latest/core.html#v1.VirtualMachineRunStrategy)

---

## 변경 파일 목록

1. `go.mod` - 의존성 추가
2. `go.sum` - 의존성 체크섬 추가
3. `cmd/main.go` - KubeVirt scheme 등록
4. `internal/controller/jeonjik_controller.go` - VirtualMachine 생성 로직 추가
5. `config/rbac/role.yaml` - RBAC 권한 추가

---

## 검증 체크리스트

- [x] 코드 컴파일 성공
- [x] Linter 오류 없음
- [x] RBAC 권한 추가됨
- [x] Scheme 등록 완료
- [x] Deprecation 경고 해결 (RunStrategy 사용)
- [x] VirtualMachine 업데이트 로직 제거
- [x] Owner Reference 설정 확인
- [ ] 실제 클러스터에서 테스트 완료
- [ ] OS 이미지 매핑 확인
- [ ] GPU 지원 테스트
- [ ] Owner Reference 동작 확인 (자동 삭제)
- [ ] KubeVirt 컨트롤러와 충돌 없음 확인

---

**작성자**: AI Assistant  
**리뷰 일자**: 2025년  
**버전**: 2.0  
**최종 업데이트**: VirtualMachine 업데이트 로직 제거, RunStrategy 변경 반영
