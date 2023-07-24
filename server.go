package main

import (
	"fmt"
	"io"
	"net"
	"runtime"
	"sync"
	"time"
)

type Server struct {
	Ip   string
	Port int

	//在线用户的列表
	OnlineMap map[string]*User
	mapLock   sync.RWMutex

	//消息广播的channel
	Message chan string
}

// 创建一个server的接口
func NewServer(ip string, port int) *Server {
	server := &Server{
		Ip:        ip,
		Port:      port,
		OnlineMap: make(map[string]*User),
		Message:   make(chan string),
	}

	return server
}

// 监听Message广播消息channel的goroutine，一旦有消息就发送给全部的在线User
func (server *Server) ListenMessager() {
	for {
		msg := <-server.Message

		//将msg发送给全部的在线User
		server.mapLock.Lock()
		for _, cli := range server.OnlineMap {
			cli.C <- msg
		}
		server.mapLock.Unlock()
	}
}

// 广播消息的方法
func (server *Server) BroadCast(user *User, msg string) {
	sendMsg := "[" + user.Addr + "]" + user.Name + ":" + msg

	server.Message <- sendMsg
}

func (server *Server) Handler(conn net.Conn) {
	//...当前链接的业务
	//fmt.Println("链接建立成功")

	user := NewUser(conn, server)

	user.Online()

	//监听用户是否活跃channel
	isLive := make(chan bool)

	//接受客户端发送的消息
	go func() {
		buf := make([]byte, 4096)
		for {
			n, err := conn.Read(buf)
			if n == 0 {
				user.Offline()
				runtime.Goexit()
			}

			if err != nil && err != io.EOF {
				fmt.Println("Conn Read err:", err)
				return
			}

			//提取用户的消息(去除'\n')
			msg := string(buf[:n-1])

			//用户针对msg进行消息处理
			user.DoMessage(msg)

			//用户的任意消息，代表当前用户是活跃状态
			isLive <- true
		}
	}()

	//当前handler阻塞
	for {
		select {
		case <-isLive:
			//检测到当前用户活跃，打破下面定时case的阻塞，重新执行一遍select达到重新定时的效果
		case <-time.After(time.Second * 300):
			//已经超时，将当前的User强制关闭
			user.C <- "You're being taken offline\n"

			//关闭channel
			close(user.C)
			//关闭连接
			//conn.Close()

			return
		}
	}
}

// 启动服务器的接口
func (server *Server) Start() {
	//socket listen
	listener, err := net.Listen("tcp", fmt.Sprintf("%s:%d", server.Ip, server.Port))
	if err != nil {
		fmt.Println("net.Listen err:", err)
		return
	}
	//close listen socket
	defer listener.Close()

	//启动监听Message的goroutine
	go server.ListenMessager()

	for {
		//accept
		conn, err := listener.Accept()
		if err != nil {
			fmt.Println("listener accept err:", err)
			continue
		}

		//do handler
		go server.Handler(conn)
	}
}
