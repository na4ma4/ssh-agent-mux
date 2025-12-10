package muxagent

import (
	"fmt"

	"google.golang.org/protobuf/proto"
)

type genericProtoMessage[T any] interface {
	proto.Message
	*T
}

func HandleExtensionProto[
	T any, P any, TT genericProtoMessage[T], PP genericProtoMessage[P],
](
	contents []byte, handler func(*T) (*P, error),
) ([]byte, error) {
	var msgT T
	msg := TT(&msgT)
	if err := proto.Unmarshal(contents, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	outMsgPtr, err := handler(&msgT)
	if err != nil {
		return nil, fmt.Errorf("failed to handle message: %w", err)
	}

	responseBytes, err := proto.Marshal(PP(outMsgPtr))
	if err != nil {
		return nil, fmt.Errorf("failed to marshal response: %w", err)
	}
	return responseBytes, nil
}

func HandleExtensionProtoInvert[
	T any, P any, TT genericProtoMessage[T], PP genericProtoMessage[P],
](
	in *T, handler func([]byte) ([]byte, error),
) (*P, error) {
	var inBytes []byte
	{
		var err error
		inBytes, err = proto.Marshal(TT(in))
		if err != nil {
			return nil, fmt.Errorf("failed to marshal response: %w", err)
		}
	}

	var outBytes []byte
	{
		var err error
		outBytes, err = handler(inBytes)
		if err != nil {
			return nil, fmt.Errorf("failed to handle message: %w", err)
		}
	}

	var msgP P
	msg := PP(&msgP)
	if err := proto.Unmarshal(outBytes, msg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal message: %w", err)
	}

	return msg, nil
}
