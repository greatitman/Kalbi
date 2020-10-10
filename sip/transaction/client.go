package transaction

/* 17.1 Client Transaction

   The client transaction provides its functionality through the
   maintenance of a state machine.

   The TU communicates with the client transaction through a simple
   interface.  When the TU wishes to initiate a new transaction, it
   creates a client transaction and passes it the SIP request to send
   and an IP address, port, and transport to which to send it.  The
   client transaction begins execution of its state machine.  Valid
   responses are passed up to the TU from the client transaction.

   There are two types of client transaction state machines, depending
   on the method of the request passed by the TU.  One handles client
   transactions for INVITE requests.  This type of machine is referred
   to as an INVITE client transaction.  Another type handles client
   transactions for all requests except INVITE and ACK.  This is
   referred to as a non-INVITE client transaction.  There is no client
   transaction for ACK.  If the TU wishes to send an ACK, it passes one
   directly to the transport layer for transmission.

   The INVITE transaction is different from those of other methods
   because of its extended duration.  Normally, human input is required
   in order to respond to an INVITE.  The long delays expected for
   sending a response argue for a three-way handshake.  On the other
   hand, requests of other methods are expected to complete rapidly.
   Because of the non-INVITE transaction's reliance on a two-way
   handshake, TUs SHOULD respond immediately to non-INVITE requests. */

import (
	"github.com/KalbiProject/Kalbi/log"
	"github.com/KalbiProject/Kalbi/sip/message"
	"github.com/KalbiProject/Kalbi/sip/method"
	"github.com/KalbiProject/Kalbi/transport"
	"github.com/looplab/fsm"
	"time"
)

const (
	clientInputRequest      = "client_input_request"
	clientInput1xx          = "client_input_1xx"
	clientInput2xx          = "client_input_2xx"
	clientInput300Plus      = "client_input_300_plus"
	clientInputTimerA       = "client_input_timer_a"
	clientInputTimerB       = "client_input_timer_b"
	clientInputTimerD       = "client_input_timer_d"
	clientInputTransportErr = "client_input_transport_err"
	clientInputDelete       = "client_input_transport_err"
)

type ClientTransaction struct {
	ID             string
	BranchID       string
	ServerTx       *ServerTransaction
	TransManager   *TransactionManager
	Origin         *message.SipMsg
	FSM            *fsm.FSM
	msgHistory     []*message.SipMsg
	ListeningPoint transport.ListeningPoint
	Host           string
	Port           string
	LastMessage    *message.SipMsg
	timerATime     time.Duration
	timerA         *time.Timer
	timerB         *time.Timer
	timerDTime     time.Duration
	timerD         *time.Timer
}

func (ct *ClientTransaction) InitFSM(msg *message.SipMsg) {

	switch string(msg.Req.Method) {
	case method.INVITE:
		ct.FSM = fsm.NewFSM("", fsm.Events{

			{Name: clientInputRequest, Src: []string{""}, Dst: "Calling"},
			{Name: clientInput1xx, Src: []string{"Calling"}, Dst: "Proceeding"},
			{Name: clientInput300Plus, Src: []string{"Proceeding"}, Dst: "Completed"},
			{Name: clientInput2xx, Src: []string{"Proceeding"}, Dst: "Terminated"},
			{Name: clientInputTransportErr, Src: []string{"Calling", "Proceeding", "Completed"}, Dst: "Terminated"},
		}, fsm.Callbacks{clientInput2xx: ct.actDelete,
			clientInput300Plus: ct.act300,
			clientInputTimerA:  ct.actResend,
			clientInputTimerB:  ct.actTransErr,
		})

	default:
		ct.FSM = fsm.NewFSM("", fsm.Events{
			{Name: clientInputRequest, Src: []string{""}, Dst: "Calling"},
			{Name: clientInput1xx, Src: []string{"Calling"}, Dst: "Proceeding"},
			{Name: clientInput300Plus, Src: []string{"Proceeding"}, Dst: "Completed"},
			{Name: clientInput2xx, Src: []string{"Proceeding"}, Dst: "Terminated"},
		}, fsm.Callbacks{})
	}
}

func (ct *ClientTransaction) SetListeningPoint(lp transport.ListeningPoint) {
	ct.ListeningPoint = lp
}

func (ct *ClientTransaction) GetBranchId() string {
	return ct.BranchID
}

func (ct *ClientTransaction) GetOrigin() *message.SipMsg {
	return ct.Origin
}

func (ct *ClientTransaction) Receive(msg *message.SipMsg) {
    ct.LastMessage = msg
	if msg.GetStatusCode() < 200 {
		ct.FSM.Event(serverInputUser1xx)
	} else if msg.GetStatusCode() < 300 {
		ct.FSM.Event(serverInputUser2xx)
	} else {
		ct.FSM.Event(serverInputUser300Plus)
	}

}

func (ct *ClientTransaction) SetServerTransaction(tx *ServerTransaction) {
	ct.ServerTx = tx
}

func (ct *ClientTransaction) GetServerTransaction() *ServerTransaction {
	return ct.ServerTx 
}

func (ct *ClientTransaction) GetLastMessage() *message.SipMsg {
	return ct.LastMessage
}

func (ct *ClientTransaction) actSend(event *fsm.Event) {
	err := ct.ListeningPoint.Send(ct.Host, ct.Port, ct.Origin.Export())
	if err != nil {
		ct.FSM.Event(clientInputTransportErr)
	}
}

func (ct *ClientTransaction) act300(event *fsm.Event) {
	log.Log.Debug("Client transaction %p, act_300", ct)
	ct.timerD = time.AfterFunc(ct.timerDTime, func() {
		ct.FSM.Event(clientInputTimerD)

	})

}

func (ct *ClientTransaction) actTransErr(event *fsm.Event) {
	log.Log.Error("Transport error for transactionID : " + ct.BranchID)
	ct.FSM.Event(clientInputDelete)
}

func (ct *ClientTransaction) actDelete(event *fsm.Event) {
	ct.TransManager.DeleteTransaction(string(ct.Origin.Via[0].Branch))
}

func (ct *ClientTransaction) actResend(event *fsm.Event) {
	log.Log.Debug("Client transaction %p, act_resend", ct)
	ct.timerATime *= 2
	ct.timerA.Reset(ct.timerATime)
	ct.Resend()
}

func (ct *ClientTransaction) Resend() {
	err := ct.ListeningPoint.Send(ct.Host, ct.Port, ct.Origin.Export())
	if err != nil {
		ct.FSM.Event(clientInputTransportErr)
	}
}

func (ct *ClientTransaction) StatelessSend(msg *message.SipMsg, host string, port string) {
	err := ct.ListeningPoint.Send(ct.Host, ct.Port, ct.Origin.Export())

	if err != nil {
		log.Log.Error("Transport error for transactionID : " + ct.BranchID)
	}

}

func (ct *ClientTransaction) Send(msg *message.SipMsg, host string, port string) {
	defer ct.FSM.Event(serverInputRequest)
	ct.Origin = msg
	ct.Host = host
	ct.Port = port
	ct.timerATime = T1

	//Retransmition timer
	ct.timerA = time.AfterFunc(ct.timerATime, func() {
		ct.FSM.Event(clientInputTimerA)
	})

	//timeout timer
	ct.timerB = time.AfterFunc(64*T1, func() {
		ct.FSM.Event(clientInputTimerB)
	})

	err := ct.ListeningPoint.Send(ct.Host, ct.Port, ct.Origin.Export())
	if err != nil {
		ct.FSM.Event(serverInputTransportErr)
	}

}
