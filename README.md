## AutoStart
1. 该工具提供延迟执行程序功能，支持开机自启。  
2. 提供基础执行程序方式。
3. 提供指定用户执行程序，例如以管理员权限执行。

## 配置文件
```json
[
  {
    "mode": 1,              // 基础方式
    "wait": true,           // 等待子程序
    "name": "notepad.exe",  // 为程序名时会从环境变量里寻找,可指定绝对路径
    "argv": "c:\\1.txt",    // 命令行参数,空格和转义按照需要填写
    "env": [                // 附带环境变量
      "OS=Windows",
      "ARCH=amd64"
    ],
    "dir": "C:\\",            // 运行的起始目录
    "stdin": "C:\\in.txt",    // 标准输入,为文件则时文件内容,否则为字符串输入
    "stdout": "C:\\out.txt",  // 标准输出,不是文件则使用默认标准输出
    "stderr": "C:\\err.txt",  // 标准错误,不是文件则使用默认标准错误
    "delay": 1,               // 延迟运行秒数
    "hide": false             // 隐藏窗口
  },
  {
    "mode": 2,                // 使用lsrunase.exe方式运行
    "user": "administrator",  // 用户名,填这个一般为管理员权限运行
    "password": "7Ft9hvgH7bvLibW3XQ==", // 密码,使用LSencrypt.exe进行加密
    "domain": "Mydomain",               // 域
    "command": "notepad.exe c:\\2.txt", // 命令行参数,包含可执行程序
    "runpath": "c:\\",                  // 运行起始目录
    "delay": 2,                         // 延迟运行秒数
    "hide": false                       // 隐藏窗口
  }
]
```

## 使用方法
1. 执行`.\AutoStart.exe -reg add -c C:\config.json`可以设置开机启动。  
2. 执行`.\AutoStart.exe -c C:\config.json`可以测试运行结果。
