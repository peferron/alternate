# alternate [![Build Status](https://drone.io/github.com/peferron/alternate/status.png)](https://drone.io/github.com/peferron/alternate/latest) [![Coverage Status](https://coveralls.io/repos/peferron/alternate/badge.png?branch=master)](https://coveralls.io/r/peferron/alternate?branch=master)

`alternate` is a simple CLI tool for running a command with alternating parameters. Used together with a reverse proxy, it makes zero-downtime upgrade of web servers easy.

## Installation

With [Go](http://golang.org/) installed, and `GOPATH/bin` added to your `PATH`:

```shell
$ go get github.com/peferron/alternate
```

Then verify that `alternate` is installed:

```shell
$ alternate
```

## Usage

```shell
$ alternate <command> <parameters...> <overlap>
```

- `command` is the command to run, with the substring `%alt` acting as placeholder for the rotated parameter. 
- `parameters...` is a space-separated list of parameters to rotate through after receiving a USR1 signal.
- `overlap` is the delay between starting the next command, and sending a TERM signal to the previous command.

## Example

To run `/home/me/myserver` alternatively on ports 3000 and 3001, with 15 seconds of overlap:

```shell
$ alternate "/home/me/myserver 127.0.0.1:%alt" 3000 3001 15s
```

1. `alternate` picks the first parameter `3000` to execute the command `/home/me/myserver 127.0.0.1:3000`. Then `alternate` waits for a USR1 signal.
2. When a USR1 signal is received, `alternate` picks the second parameter `3001` to execute the second command `/home/me/myserver 127.0.0.1:3001`. Then `alternate` waits 15 seconds.
3. When the 15 seconds are over, if the command from step 2 is still running, `alternate` sends a TERM signal to the command from step 1. Then `alternate` waits for a USR1 signal.
4. When a USR1 signal is received, `alternate` loops back to the first parameter `3000` to execute the command `/home/me/myserver 127.0.0.1:3000`. Then `alternate` waits 15 seconds.
5. When the 15 seconds are over, if the command from step 4 is still running, `alternate` sends a TERM signal to the command from step 3. Then `alternate` waits for another USR1 signal.
6. And so on.

## Zero-downtime web server upgrade

Steps for running an API server (serving JSON for example) with zero-downtime upgrades:

1. Install a reverse proxy, if you don't have one already. The cool thing with using a reverse proxy is that it also lets you configure static file serving, caching headers and so on very easily, without having to bake all this functionality into your API server. This setup uses [nginx](http://nginx.org/) as reverse proxy, but [Apache](https://httpd.apache.org/), [HAProxy](http://www.haproxy.org/) and many others would do the job as well.

1. Edit `nginx.conf` to run nginx as a reverse proxy to your API server:

    ```shell
    upstream myserver {
        server 127.0.0.1:3000 max_fails=1 fail_timeout=5s;
        server 127.0.0.1:3001 max_fails=1 fail_timeout=5s;
        keepalive 1;
    }

    server {
        listen 80;
        location /api {
            proxy_pass http://myserver;
            proxy_http_version 1.1;
            proxy_set_header Connection "";
        }
    }
    ```

2. Make sure your API server stops gracefully. Stopping gracefully means that when your API server receives a TERM signal, it should close the listening socket, then finishing processing all active requests before exiting. Go servers can achieve this very easily using the [graceful](https://github.com/stretchr/graceful) package.
3. Start your API server by running this command:

    ```shell
    $ alternate "/home/me/myserver 127.0.0.1:%alt" 3000 3001 15s
    ```

    [supervisor](http://supervisord.org/) is a great tool to automatically start this command and automatically restart it after a crash.
4. Work on your API server!
5. Overwrite `/home/me/myserver` with the latest and greatest version of your API server.
6. Send a USR1 signal to `alternate`:

    ```shell
    $ pkill -USR1 -f alternate
    ```

7. **Done!** The old and new versions of your API server will run concurrently for 15s, then the new version will take over completely, all without a hitch. Next time you want to update to a newer version, simply repeat steps 6 and 7.
