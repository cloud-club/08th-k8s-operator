# Kubernetes Resource Cleanup Operator - 배포 가이드

## 개요
이 Operator는 Kubernetes 클러스터 내 불필요한 리소스를 정책에 따라 자동으로 정리하여 자원을 효율적으로 관리합니다.

## 주요 기능

### 지원하는 리소스
- **Pod**: 실패한 Pod (CrashLoopBackOff, ImagePullBackOff 등)
- **PersistentVolume**: 사용되지 않는 PV (Released, Available 상태)

### 핵심 기능
- Cron 스케줄 기반 자동 실행 (기본: 매일 오전 6시)
- Dry-run 모드: 실제 삭제 없이 로그만 출력
- 승인 모드: 삭제 전 수동 승인 필요
- 네임스페이스 필터링 (포함/제외)
- 상세한 로깅 및 Status 업데이트

## 사전 요구사항

### 1. 도구 설치
- Go 1.21+
- Docker
- kubectl
- operator-sdk v1.41.0+
- Kubernetes 클러스터 v1.31+ (RBAC 활성화)

### 2. 클러스터 접근
```bash
# 클러스터 연결 확인
kubectl cluster-info
kubectl get nodes
```

## 설치 방법

### 1. CRD 설치
```bash
# CRD manifest 생성
make manifests

# CRD 설치
make install

# 설치 확인
kubectl get crd cleanuppolicies.cleanup.cloudclub.com
```

### 2. Operator 빌드 및 배포

#### 로컬 테스트 (클러스터 외부에서 실행)
```bash
# Operator를 로컬에서 실행
make run
```

#### 클러스터에 배포
```bash
# Docker 이미지 빌드 및 푸시
make docker-build docker-push IMG=quay.io/beengineer/cloudclub/cleanup-operator:v1.0.0

# Operator 배포
make deploy IMG=quay.io/beengineer/cloudclub/cleanup-operator:v1.0.0

# 배포 확인
kubectl get deployment -n monitoring-been-system
kubectl get pods -n monitoring-been-system
```

## 사용 방법

### 1. 기본 정책 배포 (권장)
```bash
# Dry-run 모드로 먼저 테스트
kubectl apply -f config/samples/cleanup_v1alpha1_cleanuppolicy_dryrun.yaml

# 로그 확인
kubectl logs -n monitoring-been-system deployment/monitoring-been-controller-manager -f

# 문제가 없으면 기본 정책 적용
kubectl apply -f config/samples/cleanup_v1alpha1_cleanuppolicy_basic.yaml
```

### 2. 정책 확인
```bash
# 정책 목록 조회
kubectl get cleanuppolicies

# 정책 상세 정보
kubectl describe cleanuppolicy cleanuppolicy-basic

# Status 확인
kubectl get cleanuppolicy cleanuppolicy-basic -o yaml | grep -A 20 status:
```

### 3. 정책 커스터마이징

```yaml
apiVersion: cleanup.cloudclub.com/v1alpha1
kind: CleanupPolicy
metadata:
  name: my-cleanup-policy
spec:
  # 스케줄 (Cron 형식)
  schedule: "0 6 * * *"  # 매일 오전 6시

  # Dry-run 모드 (테스트용)
  dryRun: false

  # 수동 승인 필요 여부
  requireApproval: true

  # 제외할 네임스페이스
  excludeNamespaces:
    - kube-system
    - kube-public
    - kube-node-lease
    - production

  # Pod 정리 정책
  podPolicies:
    enabled: true

    # 실패한 Pod 정리
    failedPods:
      enabled: true
      states:
        - CrashLoopBackOff
        - ImagePullBackOff
        - Error
        - Failed
      minAge: "3h"  # 3시간 경과 후 정리

  # PV 정리 정책
  pvPolicies:
    enabled: true
    minAge: "14d"  # 14일 경과 후 정리
    states:
      - Released
      - Available
```

### 4. 승인 대기 리소스 확인

`requireApproval: true`로 설정한 경우:

```bash
# 승인 대기 목록 확인
kubectl get cleanuppolicy cleanuppolicy-basic -o jsonpath='{.status.pendingApproval}' | jq

# 수동으로 리소스 삭제 (승인)
kubectl delete pod <pod-name> -n <namespace>
kubectl delete pv <pv-name>
```

## 샘플 정책

### 1. cleanup_v1alpha1_cleanuppolicy_basic.yaml
- 매일 오전 6시 실행
- 승인 모드 활성화
- 시스템 네임스페이스 제외
- **권장: 프로덕션 환경용**

### 2. cleanup_v1alpha1_cleanuppolicy_aggressive.yaml
- 하루 2회 실행 (오전 6시, 오후 6시)
- 자동 삭제 (승인 불필요)
- 더 짧은 임계값 (1시간, 7일)
- **주의: 개발/테스트 환경용**

### 3. cleanup_v1alpha1_cleanuppolicy_dryrun.yaml
- 5분마다 실행
- Dry-run 모드 (삭제 안함)
- 특정 네임스페이스만 대상
- **권장: 초기 테스트용**

## 모니터링

### 1. Operator 로그 확인
```bash
# Controller 로그
kubectl logs -n monitoring-been-system deployment/monitoring-been-controller-manager -f

# 특정 Pod 로그
kubectl logs -n monitoring-been-system <pod-name> -c manager
```

### 2. 이벤트 확인
```bash
# CleanupPolicy 관련 이벤트
kubectl get events --field-selector involvedObject.kind=CleanupPolicy

# 전체 이벤트
kubectl get events -A
```

### 3. Metrics (추후 확장)
Prometheus가 설치되어 있다면 다음 메트릭을 수집합니다:
- `cleanup_operator_resources_cleaned_total`: 정리된 리소스 수
- `cleanup_operator_execution_duration_seconds`: 실행 시간
- `cleanup_operator_execution_total`: 실행 횟수
- `cleanup_operator_pending_approval_resources`: 승인 대기 리소스 수

## 문제 해결

### 1. Operator가 시작되지 않는 경우
```bash
# Pod 상태 확인
kubectl get pods -n monitoring-been-system
kubectl describe pod <pod-name> -n monitoring-been-system

# RBAC 권한 확인
kubectl get clusterrole monitoring-been-manager-role
kubectl get clusterrolebinding monitoring-been-manager-rolebinding
```

### 2. 정책이 실행되지 않는 경우
```bash
# CRD 설치 확인
kubectl get crd cleanuppolicies.cleanup.cloudclub.com

# 정책 Status 확인
kubectl get cleanuppolicy <policy-name> -o yaml

# Controller 로그 확인
kubectl logs -n monitoring-been-system deployment/monitoring-been-controller-manager
```

### 3. 리소스가 삭제되지 않는 경우
- `dryRun: true`인지 확인
- `requireApproval: true`인지 확인 (pendingApproval 확인)
- 네임스페이스 필터링 설정 확인
- 임계값 (minAge) 확인

## 안전 가이드라인

### 1. 초기 배포 시
1. Dry-run 모드로 먼저 테스트
2. 테스트 네임스페이스에서만 실행
3. 로그를 통해 어떤 리소스가 삭제 대상인지 확인
4. 문제 없으면 승인 모드로 전환
5. 안정화되면 자동 삭제 모드 고려

### 2. 프로덕션 환경
- `requireApproval: true` 사용 권장
- 중요 네임스페이스는 `excludeNamespaces`에 추가
- 넉넉한 임계값 설정 (minAge: 3h 이상)
- 정기적인 로그 모니터링

### 3. 백업
중요한 리소스가 실수로 삭제될 수 있으므로:
- Velero 등으로 클러스터 백업
- PV는 ReclaimPolicy 확인
- 중요 Pod는 적절한 라벨로 구분

## 제거 방법

```bash
# 정책 삭제
kubectl delete cleanuppolicy --all

# Operator 삭제
make undeploy

# CRD 삭제
make uninstall
```

## 향후 확장 계획

1. **Idle Pod 모니터링**: Prometheus metrics-server 연동
2. **PV I/O 모니터링**: node-exporter 연동
3. **알림 기능**: Slack, Email 알림
4. **웹 대시보드**: 삭제 대상 리소스 시각화
5. **추가 리소스 타입**: ConfigMap, Secret, Service 등

## 참고 자료

- [Operator SDK Documentation](https://sdk.operatorframework.io/)
- [Kubebuilder Book](https://book.kubebuilder.io/)
- [Kubernetes API Reference](https://kubernetes.io/docs/reference/)
- [Cron Expression Guide](https://crontab.guru/)

## 라이선스
Apache License 2.0
