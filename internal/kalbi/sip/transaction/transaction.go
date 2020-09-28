package transaction

import (
	"github.com/marv2097/siprocket"
	"github.com/looplab/fsm"
	"Kalbi/internal/kalbi/sip/message"
	"Kalbi/internal/kalbi/log"
	"Kalbi/internal/kalbi/transport"
	"Kalbi/internal/kalbi/sdp"
	//"fmt"
)
//import "github.com/davecgh/go-spew/spew"




type Transaction interface {
    Receive(msg *siprocket.SipMsg)
}


type ClientTransaction struct {
	ID string
	FSM *fsm.FSM
	msg_history[] *siprocket.SipMsg
}

func (ct *ClientTransaction) Receive(msg *siprocket.SipMsg){
	 ct.msg_history = append(ct.msg_history, msg)
}




type ServerTransaction struct {
	ID string
	FSM *fsm.FSM
	msg_history[] *siprocket.SipMsg
}

func (st *ServerTransaction) Receive(msg *siprocket.SipMsg){
	st.msg_history = append(st.msg_history, msg)
	if  string(msg.Req.Method) == "ACK" {
        return
	}
	if string(msg.Req.Method) == "CANCEL" ||  string(msg.Req.Method) == "BYE" {
		response := message.NewResponse(200, msg)
		port := string(msg.Contact.Port)
		transport.UdpSend(string(msg.Contact.Host), string(port), response)
	}else if st.FSM.Current() == "" {
		st.FSM.Event("Proceeding")
	    response := message.NewResponse(100, msg)
		port := msg.Contact.Port
		log.Log.Info("returning response to : " + string(msg.Contact.Host) + ":" + string(port))
		transport.UdpSend(string(msg.Contact.Host), string(port), response)
		sdp.HandleSdp(msg.Sdp)		
		response = message.NewResponse(200, msg)
		transport.UdpSend(string(msg.Contact.Host), string(port), response)

	}

}

func (st *ServerTransaction) Run(e *fsm.Event){
	
		log.Log.Info(e)
		//st.FSM.Event("open")
  
}







