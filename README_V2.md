# poc-operator-sdk

Install operator-sdk:

Para usuarios mac ou linux:
https://sdk.operatorframework.io/docs/installation/

Para usuarios windows:
fazer o build do repo: https://github.com/operator-framework/operator-sdk

Criar a estrutura inicial do projeto:

- criar uma pasta vazia
```
mkdir poc-operator-sdk
cd poc-operator-sdk
```

Utilizar o operator-sdk init 
no --domain insira o domain dos groups
--repo insira o module no gomod
```
operator-sdk init --domain example.com --repo github.com/viniciusCSreis/poc-operator-sdk
```

Apos criar a estrutura inicial do projeto vamos adicionar um api ou seja um crd
```
operator-sdk create api --group pipeline --version v1alpha1 --kind Pipeline --resource --controller
```

Agora que o seu projeto vou gerado e já temos uma api, vamos alterar os
valores do arquivo: `api/pipeline_types.go` onde definiremos nosso CRD.

altere para:

```go
type PipelineEnvs struct {
    //Name env name
    Name string `json:"name"`
    //Value env value
    Value string `json:"value"`
}

// PipelineSpec defines the desired state of Pipeline
type PipelineSpec struct {
    //Envs to run pipeline
    Envs []PipelineEnvs `json:"envs"`
    //Timeout pipeline timeout in seconds
    Timeout int `json:"timeout"`
}

// PipelineStatus defines the observed state of Pipeline
type PipelineStatus struct {
    // Phase pipeline phase: [pending, running, completed]
    Phase string `json:"phase"`
    // Logs logs of a finished pipeline
    Logs string `json:"logs"`
}
```

Gere os arquivos de config:
```
make generate manifests
```

Para verificar se realmente gerou acesse o arquivos acesse:
`config/crd/bases/pipeline.example.com_pipelines.yaml` e verrifique se existe
em spec possue a properties Envs,Timeout e em status a properties Phase e Logs. 
Tambem é possivel observar que o comentario acima da variavel
nas structs geram o campo description das properties no crd.

Agora que você já definiou o CRD precisamos mudar o comportamento do
operator para após a criação da CRD criar uma pod, esperar a pod ficar
pronta e salvar os logs no CRD.

Para mudar o comportamento do operator basta mudar o arquivo
`controllers/pipeline_controller.go`

Primeiro devemos alterar a função SetupWithManager nessa função definimos
os eventos que chamaram a função Reconcile, por padrão está confiurado
para qualquer alteração no objeto pipeline chamar a função Reconcile
mas como tambem iremos criar um pod vamos alterar essa função para ouvir
eventos de pod criadas pela pipeline:

```go
func (r *PipelineReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&pipelinev1alpha1.Pipeline{}).
		Owns(&v1.Pod{}).
		Complete(r)
}
```

para utilizar o objeto `v1.Pod` basta adicionar a dependência:
```
go get k8s.io/api@v0.22.1
go mod tidy
```

Agora que já mudamos o SetupWithManager podemos colocar a nossa logica na
funçao Reconcile:

```go
func (r *PipelineReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("Reconcile loop")

	// Fetch the Pipeline instance
	pipeline := &pipelinev1alpha1.Pipeline{}
	err := r.Get(ctx, req.NamespacedName, pipeline)
	if err != nil {
		// Error reading the object - requeue the request.
		logger.Error(err, "Failed to get Pipeline")
		return ctrl.Result{}, err
	}

	// Check if the pod already exists, if not create a new one
	pod := &v1.Pod{}
	err = r.Get(ctx, types.NamespacedName{Name: pipeline.Name, Namespace: pipeline.Namespace}, pod)
	if err != nil && errors.IsNotFound(err) {
		err = r.createPod(ctx, pipeline)
		if err != nil {
			return ctrl.Result{}, err
		}
		// New Pod created successfully - return and requeue after 1s
		return ctrl.Result{RequeueAfter: time.Second}, nil
	} else if err != nil {
		logger.Error(err, "Failed to get Pod")
		return ctrl.Result{}, err
	}

	logger.Info(fmt.Sprintf("pod %s have status %s", pod.Name, pod.Status.Phase))

	switch pod.Status.Phase {
	case RunningPhase:
		err = r.processRunning(ctx, pipeline)
	case SucceededPhase:
		err = r.processSuccess(ctx, pipeline, pod)
	case FailedPhase:
		err = r.processFailed(ctx, pipeline)
	}
	return ctrl.Result{}, err

}
```
O codigo completo pode ser encontrado em: 
`controllers/pipeline_controller.go`


