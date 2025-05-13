
1. 自定义调度插件
2. [注册调度插件 simulator/cmd/scheduler/scheduler.go:17](https://github.com/kubernetes-sigs/kube-scheduler-simulator/blob/master/simulator/docs/debuggable-scheduler.md)
3. [修改配置文件 scheduler.yaml](https://pkg.go.dev/sigs.k8s.io/kube-scheduler-simulator/docs/how-to-use-custom-plugins#section-readme)


> 报错 unknown field "recordFilePath", unknown field "replayEnable":
simulator/config.yaml中注释对应字段 