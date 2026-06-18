package tools
// Defines the Average Buffer
type AvgBuffer struct {
	data []float64
	pos  int
	size int
}

// Returns the Average of all the floats in the buffer
func (self AvgBuffer) Average() (avg float64) {
	for _, value := range self.data {
		avg += value
	}
	avg /= float64(len(self.data))
	return
}

// Adds a number to the average buffer
func (self *AvgBuffer) Add(add float64) {
	if self.size <= 0 {
		self.size = 25
	}
	if self.pos < 0 || self.pos >= self.size {
		self.pos = 0
	}
	for self.pos >= len(self.data) {
		self.data = append(self.data, add)
	}
	self.data[self.pos] = add
	self.pos++
}