package services

import "context"

type MqService struct {
	Publisher TransfersPublisher
}

type MqRepository interface {
	Read(ctx context.Context) (string, error)
}

func NewMqService(publisher TransfersPublisher) *MqService {
	return &MqService{
		Publisher: publisher,
	}
}

func (s *MqService) Read(ctx context.Context) (string, error){
	result, err := s.Publisher.Read()

	if err != nil {
		return "", err
	}

	return result, nil
}