# Tutoring API

To build this API in go (dev environment only):

```sh
$ sudo su -
$ cd /var/www/api
$ systemctl stop tutoring-api
$ rm -f api
$ git checkout dev
$ git pull
$ go get -v
$ go build -o api main.go
$ systemctl start tutoring-api
$ netstat -anp |grep 5050
```


