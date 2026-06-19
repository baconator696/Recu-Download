package avgBuffer

// Defines the Average Buffer
type avgBuffer struct {
	data []float64
	pos  int
	max  int
}

// Returns the Average of all the floats in the buffer
func (self avgBuffer) Average() (avg float64) {
	for _, value := range self.data {
		avg += value
	}
	avg /= float64(len(self.data))
	return
}

// Adds a number to the average buffer
func (self *avgBuffer) Add(add float64) {
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
func New(max int) (self avgBuffer) {
	self.max = max
	return
}
