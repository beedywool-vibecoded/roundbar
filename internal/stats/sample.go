package stats

type Sample map[string]int // keys: cpu ram gpu vram cpuTemp gpuTemp

// gpuSource fills the gpu, vram and gpuTemp keys it can provide.
type gpuSource interface {
	sample(want map[string]bool, s Sample)
	stop() // release anything running in the background; sample restarts it
}
