IMG_NAME="generic-dockerimage:local"

build-kind:
	docker build -t ${IMG_NAME} .
	docker run -it -e MSG_TO_PRINT="${USER}" ${IMG_NAME}
	kind load docker-image ${IMG_NAME}

build-k3d:
	docker build -t ${IMG_NAME} .
	docker run -it -e MSG_TO_PRINT="${USER}" ${IMG_NAME}
	k3d image import ${IMG_NAME}