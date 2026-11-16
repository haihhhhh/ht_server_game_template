// client 是有限次请求的接入示例，不是生产 SDK，也不自动重试业务操作。
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/golang/protobuf/proto"
	"github.com/gorilla/websocket"
	"github.com/haihhhhh/ht_server_game_template/common/gamedata"
	pb "github.com/haihhhhh/ht_server_game_template/common/protocol/generated"
	"github.com/topfreegames/pitaya/v2/conn/codec"
	"github.com/topfreegames/pitaya/v2/conn/message"
	"github.com/topfreegames/pitaya/v2/conn/packet"
	"github.com/topfreegames/pitaya/v2/protos"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
func run() error {
	address := flag.String("url", "ws://127.0.0.1:3250/", "网关地址")
	zone := flag.String("zone", "default", "逻辑区服")
	directoryOnly := flag.Bool("directory-only", false, "只登录和查看目录，不进入角色")
	boss := flag.Bool("boss", false, "调用公共 Boss 示例，需要 world 服务")
	useExp := flag.Int64("use-exp-item", 0, "使用经验道具数量，零表示不调用")
	flag.Parse()
	catalog, err := pb.NewProtocolCatalog()
	if err != nil {
		return err
	}
	conn, _, err := (&websocket.Dialer{HandshakeTimeout: 3 * time.Second}).Dial(*address, nil)
	if err != nil {
		return err
	}
	defer conn.Close()
	conn.SetReadLimit(65536)
	_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
	_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	write := func(kind packet.Type, data []byte) error {
		b, e := codec.NewPomeloPacketEncoder().Encode(kind, data)
		if e != nil {
			return e
		}
		return conn.WriteMessage(websocket.BinaryMessage, b)
	}
	decoder := codec.NewPomeloPacketDecoder()
	pending := []*packet.Packet{}
	read := func() (*packet.Packet, error) {
		for len(pending) == 0 {
			_, b, e := conn.ReadMessage()
			if e != nil {
				return nil, e
			}
			pending, e = decoder.Decode(b)
			if e != nil {
				return nil, e
			}
		}
		p := pending[0]
		pending = pending[1:]
		return p, nil
	}
	if err = write(packet.Handshake, []byte(`{"sys":{"type":"go-example","version":"1"},"user":{}}`)); err != nil {
		return err
	}
	handshake, err := read()
	if err != nil {
		return err
	}
	var response struct {
		Code int `json:"code"`
	}
	if handshake.Type != packet.Handshake || json.Unmarshal(handshake.Data, &response) != nil || response.Code != 200 {
		return errors.New("handshake rejected")
	}
	if err = write(packet.HandshakeAck, nil); err != nil {
		return err
	}
	var sequence uint
	// 【示例客户端｜R01、R12】编码请求并发送，收到响应后核对请求序号和消息类型；业务无需维护这些编号。
	call := func(req, reply proto.Message) error {
		sequence++
		_ = conn.SetReadDeadline(time.Now().Add(5 * time.Second))
		_ = conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
		b, e := catalog.EncodeRequest(sequence, req)
		if e != nil {
			return e
		}
		if e = write(packet.Data, b); e != nil {
			return e
		}
		for {
			p, e := read()
			if e != nil {
				return e
			}
			if p.Type == packet.Heartbeat {
				if e = write(packet.Heartbeat, nil); e != nil {
					return e
				}
				continue
			}
			if p.Type != packet.Data {
				return errors.New("connection ended")
			}
			m, number, e := catalog.DecodeServer(p.Data)
			if e != nil {
				return e
			}
			if m.Type == message.Push {
				fmt.Println("推送消息类型：", m.Route)
				continue
			}
			if m.Type != message.Response || m.ID != sequence {
				return errors.New("unexpected response sequence")
			}
			if m.Err {
				var failure protos.Error
				if e = proto.Unmarshal(m.Data, &failure); e != nil {
					return e
				}
				return fmt.Errorf("业务请求失败：%s", failure.Code)
			}
			expected, ok := catalog.MessageID(reply)
			if !ok || expected != number {
				return errors.New("unexpected response message type")
			}
			return proto.Unmarshal(m.Data, reply)
		}
	}
	login := new(pb.LoginRep)
	if err = call(&pb.LoginReq{Credential: os.Getenv("HT_DEV_TICKET")}, login); err != nil {
		return err
	}
	fmt.Println("登录完成")
	zones := new(pb.ListZonesRep)
	if err = call(&pb.ListZonesReq{}, zones); err != nil {
		return err
	}
	for _, z := range zones.Zones {
		fmt.Printf("逻辑区：%s %s，开放：%t\n", z.Id, z.Name, z.Open)
	}
	list := new(pb.ListRolesRep)
	if err = call(&pb.ListRolesReq{Zone: *zone}, list); err != nil {
		return err
	}
	fmt.Println("角色数量：", len(list.Roles))
	if *directoryOnly {
		return nil
	}
	entered := new(pb.EnterRep)
	if err = call(&pb.EnterReq{Zone: *zone}, entered); err != nil {
		return err
	}
	state := entered.State
	if state == nil {
		return errors.New("enter response has no player state")
	}
	fmt.Printf("进入角色：%d，登录次数：%d\n", state.RoleId, state.LoginCount)
	preferences := new(pb.SetPreferencesRep)
	if err = call(&pb.SetPreferencesReq{Preferences: "sample-setting"}, preferences); err != nil {
		return err
	}
	state.Preferences = preferences.Preferences
	fmt.Println("当前设置：", state.Preferences)
	if *boss {
		result := new(pb.AttackBossRep)
		if err = call(&pb.AttackBossReq{}, result); err != nil {
			return err
		}
		fmt.Printf("Boss 剩余血量：%d，个人剩余次数：%d\n", result.BossHp, result.ChallengesLeft)
	}
	if *useExp > 0 {
		used := new(pb.UseExpItemRep)
		if err = call(&pb.UseExpItemReq{
			ItemId: gamedata.StarterExpItemID,
			Count:  *useExp,
		}, used); err != nil {
			return err
		}
		fmt.Printf("道具：%d，剩余：%d，经验：%d，等级：%d\n", used.ItemId, used.RemainingCount, used.Experience, used.Level)
	}
	left := new(pb.LeaveRep)
	if err = call(&pb.LeaveReq{}, left); err != nil {
		return err
	}
	fmt.Println("已离开，最后保存确认：", left.SaveConfirmed)
	return nil
}
