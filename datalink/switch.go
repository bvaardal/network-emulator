package datalink

type Switch struct {
	// Active ports
	connected []port

	// Discovered MAC to port mappings
	macToPort map[MAC]port

	// Channels for internal communication
	connections chan port
	frames      chan frameFromPort
}

type frameFromPort struct {
	frame Frame
	port  port
}

type port struct {
	outbound chan<- Frame
	inbound  <-chan Frame
}

type MAC uint64

const (
	BroadcastMAC MAC = 0xFFFFFFFFFFFF
)

type Frame struct {
	SourceMAC      MAC
	DestinationMAC MAC
	EtherType      uint16
	Payload        []byte
}

func NewSwitch() *Switch {
	return &Switch{
		frames:      make(chan frameFromPort),
		macToPort:   make(map[MAC]port),
		connections: make(chan port),
	}
}

func (s *Switch) Start() {
	go func() {
		for {
			select {
			case port, ok := <-s.connections:
				if !ok {
					return
				}
				s.connected = append(s.connected, port)
			case frameInfo, ok := <-s.frames:
				if !ok {
					return
				}
				s.handleFrame(frameInfo.frame, frameInfo.port)
			}
		}
	}()
}

func (s *Switch) handleFrame(frame Frame, incomingPort port) {
	// Learn the source MAC to port mapping
	s.macToPort[frame.SourceMAC] = incomingPort

	outPort, macKnown := s.macToPort[frame.DestinationMAC]
	if macKnown && frame.DestinationMAC != BroadcastMAC {
		// Forward to the specific port if known and not broadcast
		outPort.outbound <- frame
	} else {
		// Broadcast to all ports except the source
		for _, broadcastPort := range s.connected {
			if broadcastPort != incomingPort {
				broadcastPort.outbound <- frame
			}
		}
	}
}

func (s *Switch) Connect(inbound <-chan Frame) <-chan Frame {
	outbound := make(chan Frame)

	port := port{
		outbound: outbound,
		inbound:  inbound,
	}

	s.connections <- port
	// Although the port may not be fully registered yet, listening is safe
	// because frames from this port should never be sent back to it.
	go func() {
		for frame := range inbound {
			s.frames <- frameFromPort{frame, port}
		}
	}()

	return outbound
}
