# poc-operator-sdk

## Install operator-sdk

Para usuarios mac ou linux:
https://sdk.operatorframework.io/docs/installation/

Para usuarios windows:
fazer o build do repo: https://github.com/operator-framework/operator-sdk

## Criar a estrutura inicial do projeto

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

## Alterar CRD gerado

Agora que o seu projeto foi gerado e já temos uma api, vamos alterar os
valores do arquivo: `api/pipeline_types.go` onde iremos definir nosso CRD.

Altere para:

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
`config/crd/bases/pipeline.example.com_pipelines.yaml` e verifique se existe
em spec possue a properties Envs,Timeout e em status a properties Phase e Logs. 
Tambem é possivel observar que o comentario acima da variavel
nas structs geram o campo description das properties no crd.

## Mudar comportamento do operator

Agora que você já definiou o CRD precisamos mudar o comportamento do
operator para após a criação da CRD criar uma pod, esperar a pod ficar
pronta e salvar os logs no CRD.

Para mudar o comportamento do operator basta mudar o arquivo
`controllers/pipeline_controller.go`

Primeiro devemos alterar a função SetupWithManager nessa função definimos
os eventos que chamaram a função Reconcile, por padrão está configurado
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


## Gerar imagem docker da pod
A ideia é criar uma pod, esperar a pod ficar pronta e salvar os logs no CRD.
porem para criar uma pod precisamos definir qual a imagem docker a pod
vai rodar, utilizando o kind ou k3d você pode fazer o build imagem docker
localmente e importar no cluster 

k3d:

```
cd generic-dockerimage
make build-k3d
```

kind:

```
cd generic-dockerimage
make build-kind
```

## Rodar aplicação
Para rodar a aplicação:

```
make install
make run
```

Para criar um crd

```
kubectl apply -f config/samples/pipeline_v1alpha1_pipeline.yaml
```

## k8s.io/client-go

Um dos nossos objetivos é salvar os logs no CRD porem utilizando apenas o client
do controller-runtime(client fornecido pelo operator-sdk) não é possivel
chamar alguns recursos da api do K8s, portanto para pegar os logs da pod 
chamaremos a api do K8S diretamente, na main vamos criar um k8sClient:

```go
	k8sRestConfig := ctrl.GetConfigOrDie()
	k8sClient := kubernetes.NewForConfigOrDie(k8sRestConfig)
```

e passar esse k8sClient para o operator:

```go
if err = (&controllers.PipelineReconciler{
		Client:    mgr.GetClient(),
		Scheme:    mgr.GetScheme(),
		K8sClient: k8sClient,
	}).SetupWithManager(mgr); err != nil {
		setupLog.Error(err, "unable to create controller", "controller", "Pipeline")
		os.Exit(1)
	}
```

Com isso agora o operator pode pegar os logs da pod:

```go
func (r *PipelineReconciler) getPodLogs(ctx context.Context, pod *v1.Pod) (string, error) {
	opts := v1.PodLogOptions{
		Follow: true,
	}
	req := r.K8sClient.CoreV1().Pods(pod.Namespace).GetLogs(pod.Name, &opts)
	podLogs, err := req.Stream(ctx)
	if err != nil {
		return "", err
	}

	logs := ""
	scanner := bufio.NewScanner(podLogs)
	for scanner.Scan() {
		msg := scanner.Text() + "\n"
		logs += msg
	}
	return logs, nil
}
```

## Test controller

Por padrão o operator sdk cria o arquivo suite_test.go, esse arquivo tem a
responsabilidade de testar os nossos controllers.

Para realizar o teste do controller o time do operator sdk incentiva
não a criação de mocks da api do k8s mas a utilização do envtest que é um
binario que simula um ambiente k8s.

Para instalar o envtest é so utilizar o comando:
```
make envtest
```

Além disso o time do operator sdk incentiva a utilização do framework ginkgo
para a criação de testes utilizando o BDD porem para não deixar essa poc
muito complexa para realizar os testes dos controller irei utilizar a 
biblioteca padrão de test do golang.

