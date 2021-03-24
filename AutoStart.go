package main

import (
	"embed"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"golang.org/x/sys/windows/registry"
	"io"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"
)

func main() {
	regCtl := flag.String("reg", "", "[Add|Del] Startup")
	config := flag.String("c", "DelayStart.json", "config file")
	flag.Usage = func() {
		fmt.Printf(`Usage of %s:
  -c string
        config file (default "DelayStart.json")
  -reg string
        [Add|Del] Startup

Example DelayStart.json:
[
  {
    "mode": 1,
    "wait": true,
    "name": "notepad.exe",
    "argv": "c:\\1.txt",
    "env": [
      "OS=Windows",
      "ARCH=amd64"
    ],
    "dir": "C:\\",
    "stdin": "C:\\in.txt",
    "stdout": "C:\\out.txt",
    "stderr": "C:\\err.txt",
    "delay": 1,
    "hide": false
  },
  {
    "mode": 2,
    "user": "administrator",
    "password": "7Ft9hvgH7bvLibW3XQ==",
    "domain": "Mydomain",
    "command": "notepad.exe c:\\2.txt",
    "runpath": "c:\\",
    "delay": 2,
    "hide": false
  }
]`, os.Args[0])
	}
	flag.Parse()

	err := checkLsRunAs()
	if err != nil {
		log.Fatal(err)
	}

	if *regCtl != "" {
		err = handleRegistry(*regCtl, *config)
		if err != nil {
			log.Println(err)
		}
		return
	}

	task, err := openConfig(*config)
	if err != nil {
		log.Fatal(err)
	}

	wg := new(sync.WaitGroup)
	for _, v := range task {
		err = v.prepare()
		if err != nil {
			log.Println(err)
			continue
		}
		wg.Add(1)
		go v.run(wg)
	}
	wg.Wait()
	// 等5秒后推出
	time.Sleep(time.Second * 5)
}

const (
	noneMode     runMode = iota // 0:不运行,不想删配置时不让运行
	defaultMode                 // 1:默认运行
	lsrunaseMode                // 2:lsrunase运行
)

type (
	runMode    uint8
	DelayStart struct {
		Mode runMode `json:"mode"` // 运行方式

		Wait   bool     `json:"wait"`   // 是否等待运行结束
		Name   string   `json:"name"`   // 程序名,或程序路径
		Argv   string   `json:"argv"`   // 程序命令行参数
		Env    []string `json:"env"`    // 环境变量
		Dir    string   `json:"dir"`    // 起始路径
		Stdin  string   `json:"stdin"`  // 标准输入
		Stdout string   `json:"stdout"` // 标准输出
		Stderr string   `json:"stderr"` // 标准错误

		// 使用lsrunase运行,主要用于管理员权限运行
		User    string `json:"user"`
		Pass    string `json:"password"`
		Domain  string `json:"domain"`
		Command string `json:"command"`
		RunPath string `json:"runpath"`

		Delay uint64 `json:"delay"` // 延时秒数
		Hide  bool   `json:"hide"`  // 隐藏窗口
		delay time.Duration
		cmd   *exec.Cmd // 上述配置生成
	}
)

func openConfig(p string) ([]*DelayStart, error) {
	fr, err := os.Open(p)
	if err != nil {
		return nil, err
	}
	defer fr.Close()
	var d []*DelayStart
	err = json.NewDecoder(fr).Decode(&d)
	return d, err
}

func (t *DelayStart) prepare() (err error) {
	switch t.Mode {
	default:
		return fmt.Errorf("%d mode error", t.Mode)
	case noneMode:
		var name string
		if t.Name != "" {
			name = t.Name + " " + t.Argv
		} else if t.Command != "" {
			name = t.Command
		}
		return errors.New(name + " no need to run")
	case defaultMode:
		t.cmd, err = Command(t.Name, t.Argv, t.Hide)
		if err != nil {
			return
		}
		err = IsFilePathExists(t.cmd.Path, true)
		if err != nil {
			return
		}

		t.cmd.Env = os.Environ()
		for _, v := range t.Env {
			if strings.Count(v, "=") == 1 {
				t.cmd.Env = append(t.cmd.Env, strings.TrimSpace(v)) // 新增环境变量
			}
		}

		if IsFilePathExists(t.Dir, false) == nil {
			t.cmd.Dir = t.Dir // 起始路径
		}

		t.cmd.Stdin, err = os.Open(t.Stdin)
		if err != nil { /* 读取文件错误,则按照字符串读取 */
			t.cmd.Stdin = strings.NewReader(t.Stdin)
		}

		t.cmd.Stdout, err = os.OpenFile(t.Stdout, os.O_CREATE|os.O_APPEND, 0666)
		if err != nil {
			t.cmd.Stdout = os.Stdout
		}

		if t.Stdout == t.Stderr { // 标准输出和标准错误一样
			t.cmd.Stderr = t.cmd.Stdout
		} else {
			t.cmd.Stderr, err = os.OpenFile(t.Stderr, os.O_CREATE|os.O_APPEND, 0666)
			if err != nil {
				t.cmd.Stderr = os.Stderr
			}
		}
	case lsrunaseMode:
		if t.User == "" {
			return errors.New("user is nil")
		}
		str := new(strings.Builder)
		str.WriteString(" /user:")
		str.WriteString(t.User)

		if t.Pass == "" {
			return errors.New("password is nil")
		}
		str.WriteString(" /password:")
		str.WriteString(t.Pass)

		if t.Domain == "" {
			return errors.New("domain is nil")
		}
		str.WriteString(" /domain:")
		str.WriteString(t.Domain)

		if t.RunPath == "" {
			return errors.New("runpath is nil")
		}
		str.WriteString(" /runpath:")
		str.WriteString(t.RunPath)

		if t.Command == "" {
			return errors.New("command is nil")
		}
		str.WriteString(" /command:")
		str.WriteString(t.Command)
		t.cmd, err = Command(lsrunase, str.String(), t.Hide)
		if err != nil {
			return
		}
		t.cmd.Stdout = os.Stdout
		t.cmd.Stderr = os.Stderr
		t.Wait = true // 该方式需要等待
	}
	t.delay = time.Second * time.Duration(t.Delay)
	return
}

func (t *DelayStart) run(wg *sync.WaitGroup) {
	defer wg.Done() // done one task
	if t.cmd == nil {
		return
	}

	time.Sleep(t.delay)
	log.Println(t.cmd.SysProcAttr.CmdLine)

	err := t.cmd.Start()
	if err != nil {
		log.Println("start", err)
		return
	}
	if t.Wait {
		err = t.cmd.Wait()
		if err != nil {
			log.Println("wait", err)
			return
		}
	}
}

// 更方便易用的exec.Command
func Command(name, args string, hide bool) (*exec.Cmd, error) {
	if filepath.Base(name) == name {
		lp, err := exec.LookPath(name)
		if err != nil {
			return nil, err
		}
		name = lp
	}
	return &exec.Cmd{
		Path: name,
		SysProcAttr: &syscall.SysProcAttr{
			HideWindow: hide,
			CmdLine:    name + " " + args,
		},
	}, nil
}

// 判断文件或文件夹存在
func IsFilePathExists(path string, isFile bool) error {
	if path == "" {
		return errors.New("path is nil")
	}
	f, err := os.Stat(path)
	if err != nil {
		return err
	}
	if isFile != f.IsDir() {
		return nil
	}
	if isFile {
		return errors.New(path + " is dir")
	}
	return errors.New(path + " is file")
}

//go:embed lsrunase.exe
//go:embed LSencrypt.exe
var fileList embed.FS

const (
	lsrunaseName  = "lsrunase.exe"
	lsencryptName = "LSencrypt.exe"
)

var lsrunase string

// 检查两个可执行程序不存在则自动生成
func checkLsRunAs() error {
	decFile := func(name, f string) error {
		err := IsFilePathExists(f, true)
		if err == nil {
			return nil
		}
		fr, err := fileList.Open(name)
		if err != nil {
			return err
		}
		fw, err := os.Create(f)
		if err != nil {
			return err
		}
		defer fw.Close()
		_, err = io.Copy(fw, fr)
		return err
	}

	path, err := os.UserConfigDir()
	if err != nil {
		return err
	}

	lsrunase = filepath.Join(path, "janbar", lsrunaseName)
	err = decFile(lsrunaseName, lsrunase)
	if err != nil {
		return err
	}
	// 生成加密程序到运行目录
	return decFile(lsencryptName, filepath.Join(filepath.Dir(os.Args[0]), lsencryptName))
}

const (
	regPath = `SOFTWARE\Microsoft\Windows\CurrentVersion\Run`
	regName = "StartDelay"
)

// 生成开机自启
func handleRegistry(opr, config string) error {
	k, err := registry.OpenKey(registry.CURRENT_USER, regPath, registry.ALL_ACCESS)
	if err != nil {
		registry.CreateKey(registry.CURRENT_USER, regPath, registry.ALL_ACCESS)
		k, err = registry.OpenKey(registry.CURRENT_USER, regPath, registry.ALL_ACCESS)
		if err != nil { // 创建后重新打开
			return err
		}
	}
	defer k.Close()

	if strings.ToLower(opr) == "del" {
		return k.DeleteValue(regName)
	}

	str, err := filepath.Abs(os.Args[0])
	if err != nil {
		return err
	}
	startUp := new(strings.Builder)
	startUp.WriteByte('"')
	startUp.WriteString(str)
	startUp.WriteByte('"')
	if config != "" {
		cnf, err := filepath.Abs(config)
		if err != nil {
			return err
		}
		err = IsFilePathExists(cnf, true)
		if err != nil {
			return err
		}
		startUp.WriteString(" -c \"")
		startUp.WriteString(cnf)
		startUp.WriteByte('"')
	}
	return k.SetStringValue(regName, startUp.String())
}
