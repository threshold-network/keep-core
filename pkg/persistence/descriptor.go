package persistence

// dataDescriptor is the simplest possible implementation of DataDescriptor
// interface that can be used by a storage when reading data.
type dataDescriptor struct {
	name      string
	directory string
	readFunc  func() ([]byte, error)
}

func (dd *dataDescriptor) Name() string {
	return dd.name
}

func (dd *dataDescriptor) Directory() string {
	return dd.directory
}

func (dd *dataDescriptor) Content() ([]byte, error) {
	return dd.readFunc()
}
