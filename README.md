# Go-Chat

An experiment to play around with sockets and socket communication.
Also includes a lightweight experiment for a terminal graphical interface.

## Structure
### Directory
Think of this as the central point where the chat clients are registering in order to be able to discover other users.
### Client
The actual client of the chat.

## Run
In order to run it you have multiple options
### Docker Compose
To run this just execute `docker-compose up -d`.

The [docker-compose.yaml](docker-compose.yaml) file contains also a client entry but the container will not run, is there only to force the docker image creation.
After the `docker-compose` command finished you should run the following command to start it.

```shell
docker run -it --network=chat_chat-bridge --platform linux/amd64 --env USER_NAME=demo_user --env SERVER_URL=http://chat-directory:8080 chat-client1
```
In case this is not working check the following:
* `docker ps -a | grep directory` - check that the directory application is up and running
* `docker network ls | grep chat` - check that the `--network` flag from the above command is matching the one in the docker networks
* `docker images | grep client` - check that the name of the image at the end of the command above is matching the one from docker images

### Go
If you want to run this using Go just run the following:
* Server
    ```shell
    go run directory/cmd/directory/main.go
    ```
* Client
    ```shell
    export USER_NAME="user1"; export SERVER_URL=http://localhost:8080; go run client/cmd/client/main.go
    ```