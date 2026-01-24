package performance

type Service struct {
	store StoreAPI
}

func NewService(store StoreAPI) *Service {
	return &Service{store: store}
}
