package agent

import (
	"fmt"
	"net"

	"Stowaway/node"
	"Stowaway/utils"
)

//管理下行节点代码

// HandleLowerNodeConn 处理接收下级节点数据的信道
func HandleLowerNodeConn(connForLowerNode net.Conn, payloadBuffChan chan *utils.Payload, currentid, lowerid string) {
	defer func() {
		connForLowerNode.Close()
		node.NodeInfo.LowerNode.Lock()
		delete(node.NodeInfo.LowerNode.Payload, lowerid) //下级节点掉线，立即将此节点从自己的子节点列表删除
		node.NodeInfo.LowerNode.Unlock()
		offlineMess, _ := utils.ConstructPayload(utils.AdminId, "", "COMMAND", "AGENTOFFLINE", " ", lowerid, 0, currentid, AgentStatus.AESKey, false) //通知admin下级节点已经下线
		AgentStuff.ProxyChan.ProxyChanToUpperNode <- offlineMess
		close(payloadBuffChan)
	}()

	for {
		command, err := utils.ExtractPayload(connForLowerNode, AgentStatus.AESKey, currentid, false)
		if err != nil {
			return
		}
		payloadBuffChan <- command
	}
}

// HandleDataToLowerNode 管理发往下级节点的信道
func HandleDataToLowerNode() {
	for {
		proxyData := <-AgentStuff.ProxyChan.ProxyChanToLowerNode

		node.NodeInfo.LowerNode.Lock()
		if _, ok := node.NodeInfo.LowerNode.Payload[proxyData.Route]; ok { //检查此节点是否存活，防止admin误操作在已掉线的节点输入命令导致节点panic
			node.NodeInfo.LowerNode.Payload[proxyData.Route].Write(proxyData.Data)
		}
		node.NodeInfo.LowerNode.Unlock()
	}
}

// HandleDataFromLowerNode 处理来自下级节点的数据
func HandleDataFromLowerNode(connForLowerNode net.Conn, payloadBuffChan chan *utils.Payload, currentid, lowerid string) {
	for {
		if command, ok := <-payloadBuffChan; ok {
			switch command.Type {
			case "COMMAND":
				switch command.Command {
				case "RECONNID":
					var info string
					if _, ok := node.NodeInfo.LowerNode.Payload[command.CurrentId]; ok {
						if connForLowerNode.RemoteAddr().String() == "0.0.0.0:0" {
							info = fmt.Sprintf("%s:::%s", currentid, "Can't retrive ip address") //被sshtunnel连接的节点在重连时无法取到ip:port信息，可利用'shell' -> 'ipconfig/ifconfig'自行查看
						} else {
							info = fmt.Sprintf("%s:::%s", currentid, connForLowerNode.RemoteAddr().String())
						}
						proxyCommand, _ := utils.ConstructPayload(utils.AdminId, "", "COMMAND", command.Command, " ", info, 0, command.CurrentId, AgentStatus.AESKey, false)
						AgentStuff.ProxyChan.ProxyChanToUpperNode <- proxyCommand
						continue
					} else {
						proxyCommand, _ := utils.ConstructPayload(command.NodeId, command.Route, command.Type, command.Command, command.FileSliceNum, command.Info, command.Clientid, command.CurrentId, AgentStatus.AESKey, true)
						AgentStuff.ProxyChan.ProxyChanToUpperNode <- proxyCommand
						continue
					}
				case "HEARTBEAT":
					hbCommPack, _ := utils.ConstructPayload(command.CurrentId, "", "COMMAND", "KEEPALIVE", " ", " ", 0, currentid, AgentStatus.AESKey, false)
					passToLowerData := utils.NewPassToLowerNodeData()
					passToLowerData.Data = hbCommPack
					passToLowerData.Route = command.CurrentId
					AgentStuff.ProxyChan.ProxyChanToLowerNode <- passToLowerData
					continue
				default:
					proxyData, _ := utils.ConstructPayload(command.NodeId, command.Route, command.Type, command.Command, command.FileSliceNum, command.Info, command.Clientid, command.CurrentId, AgentStatus.AESKey, true)
					AgentStuff.ProxyChan.ProxyChanToUpperNode <- proxyData
				}
			case "DATA":
				proxyData, _ := utils.ConstructPayload(command.NodeId, command.Route, command.Type, command.Command, command.FileSliceNum, command.Info, command.Clientid, command.CurrentId, AgentStatus.AESKey, true)
				AgentStuff.ProxyChan.ProxyChanToUpperNode <- proxyData
			}
		} else {
			return
		}
	}
}

//管理下行节点代码结束
