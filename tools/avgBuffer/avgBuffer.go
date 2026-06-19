package avgBuffer

// Defines the Average Buffer
type AvgBuffer struct {
	data []float32
	pos  int
	max  int
}

// Returns the Average of all the floats in the buffer
func (self AvgBuffer) Average() (avg float32) {
	for _, value := range self.data {
		avg += value
	}
	avg /= float32(len(self.data))
	return
}

// Adds a number to the average buffer
func (self *AvgBuffer) Add(add float32) {
	if len(self.data) != self.max {
		self.data = append(self.data, add)
		self.pos = len(self.data)
		return
	}
	if self.pos < 0 || self.pos >= len(self.data) {
		self.pos = 0
	}
	self.data[self.pos] = add
	self.pos++
}
func New(max int) (self AvgBuffer) {
	self.max = max
	return
}
