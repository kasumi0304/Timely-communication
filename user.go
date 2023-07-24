package main

import (
	"fmt"
	"net"
	"strings"
)

type User struct {
	Name   string
	Addr   string
	C      chan string
	conn   net.Conn
	server *Server
}

// 创建一个用户的API
func NewUser(conn net.Conn, server *Server) *User {
	userAddr := conn.RemoteAddr().String()

	user := &User{
		Name:   userAddr,
		Addr:   userAddr,
		C:      make(chan string),
		conn:   conn,
		server: server,
	}

	//启动监听当前user channel消息的goroutine
	go user.ListenMessage()

	return user
}

// 用户的上线业务
func (user *User) Online() {

	//用户上线,将用户加入到onlineMap中
	user.server.mapLock.Lock()
	user.server.OnlineMap[user.Name] = user
	user.server.mapLock.Unlock()

	//广播当前用户上线消息
	user.server.BroadCast(user, "Online")
}

// 用户的下线业务
func (user *User) Offline() {

	//用户下线，将用户从onlineMap中删除
	user.server.mapLock.Lock()
	delete(user.server.OnlineMap, user.Name)
	user.server.mapLock.Unlock()

	//广播当前用户下线消息
	user.server.BroadCast(user, "Offline")

}

// 用户处理消息的业务
func (u *User) DoMessage(msg string) {
	if msg == "who" {
		//查询在线用户都有哪些
		u.server.mapLock.Lock()
		for _, user := range u.server.OnlineMap {
			onlineMsg := "[" + user.Addr + "]" + user.Name + ":is online..."
			u.C <- onlineMsg
		}
		u.server.mapLock.Unlock()

	} else if len(msg) > 7 && msg[:7] == "rename|" {
		//消息格式：rename|张三
		newName := strings.Split(msg, "|")[1]

		//判断name是否存在
		_, ok := u.server.OnlineMap[newName]
		if ok {
			u.C <- "The current user name is used\n"
		} else {
			u.server.mapLock.Lock()
			delete(u.server.OnlineMap, u.Name)
			u.server.OnlineMap[newName] = u
			u.server.mapLock.Unlock()
			u.Name = newName
			u.C <- "You have updated your username:" + u.Name + "\n"
		}
	} else if len(msg) > 4 && msg[:3] == "to|" {
		//消息格式：to|张三|消息内容

		//获取对方用户名
		remoteName := strings.Split(msg, "|")[1]
		if remoteName == "" {
			u.C <- "The message is not in the correct format. Use the \"to|JohnDoe|hello\" format.\n"
			return
		}

		//根据用户名得到对方User对象
		remoteUser, ok := u.server.OnlineMap[remoteName]
		if !ok {
			u.C <- "The user does not exist or is not online\n"
			return
		}
		//获取消息内容， 通过对方的User对象将消息内容发送过去
		content := strings.Split(msg, "|")[2]
		if content == "" {
			u.C <- "Message content is empty, please re-send\n"
			return
		}
		remoteUser.C <- u.Name + " said to you: " + content
	} else {
		u.server.BroadCast(u, msg)
	}
}

// 监听当前User channel的方法，一旦有消息，就直接发送给客户端
func (user *User) ListenMessage() {
	//当user的channel关闭后，不再监听并写入信息
	for msg := range user.C {
		_, err := user.conn.Write([]byte(msg + "\n"))
		if err != nil {
			fmt.Println(err.Error())
		}
	}

	//不监听后关闭conn
	err := user.conn.Close()
	if err != nil {
		fmt.Println(err.Error())
	}
}
