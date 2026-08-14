type AudioContextWithWebkit = typeof window & {
  webkitAudioContext?: typeof AudioContext
}

const PRINT_DURATION_MS = 5200
const CUTTER_START_DELAY_MS = 0
const SOUND_CLEANUP_GRACE_MS = 200

export function playThermalPrinterSound(): (() => void) | null {
  if (typeof window === 'undefined') return null

  const AudioContextConstructor = window.AudioContext
    || (window as AudioContextWithWebkit).webkitAudioContext
  if (!AudioContextConstructor) return null

  const context = new AudioContextConstructor()
  const startAt = context.currentTime + 0.02
  const feedDuration = PRINT_DURATION_MS / 1000
  const cutterStartDelay = CUTTER_START_DELAY_MS / 1000
  const mechanicalCloseAt = startAt + feedDuration + cutterStartDelay + 0.14
  const mechanicalCloseDuration = 0.68
  const stopAt = mechanicalCloseAt + mechanicalCloseDuration
  const sources: AudioScheduledSourceNode[] = []

  const printCompressor = context.createDynamicsCompressor()
  const master = context.createGain()
  printCompressor.threshold.value = -15
  printCompressor.knee.value = 9
  printCompressor.ratio.value = 4
  printCompressor.attack.value = 0.002
  printCompressor.release.value = 0.12
  master.gain.setValueAtTime(0.96, startAt)
  master.gain.setValueAtTime(0.96, startAt + feedDuration)
  master.gain.exponentialRampToValueAtTime(0.0001, stopAt)
  master.connect(printCompressor)
  printCompressor.connect(context.destination)

  const completionCompressor = context.createDynamicsCompressor()
  const completionGain = context.createGain()
  completionCompressor.threshold.value = -12
  completionCompressor.knee.value = 8
  completionCompressor.ratio.value = 4
  completionCompressor.attack.value = 0.002
  completionCompressor.release.value = 0.18
  completionGain.gain.setValueAtTime(1.18, mechanicalCloseAt)
  completionGain.gain.exponentialRampToValueAtTime(0.0001, stopAt)
  completionCompressor.connect(completionGain)
  completionGain.connect(context.destination)

  const sampleCount = Math.ceil(context.sampleRate * feedDuration)
  const noiseBuffer = context.createBuffer(1, sampleCount, context.sampleRate)
  const noiseData = noiseBuffer.getChannelData(0)
  let previousSample = 0
  for (let index = 0; index < noiseData.length; index += 1) {
    const white = (Math.random() * 2) - 1
    previousSample = (previousSample * 0.84) + (white * 0.16)
    noiseData[index] = previousSample
  }

  const noise = context.createBufferSource()
  const noiseHighPass = context.createBiquadFilter()
  const noiseLowPass = context.createBiquadFilter()
  const noiseGain = context.createGain()
  noise.buffer = noiseBuffer
  noiseHighPass.type = 'highpass'
  noiseHighPass.frequency.value = 560
  noiseLowPass.type = 'lowpass'
  noiseLowPass.frequency.value = 4200
  noiseGain.gain.setValueAtTime(0.018, startAt)

  // A dense brush of short strokes gives the feed the dry, mechanical rhythm
  // of a vintage typewriter carriage rather than an electronic buzz.
  for (let offset = 0; offset < feedDuration - 0.05; offset += 0.052) {
    const pulseAt = startAt + offset
    noiseGain.gain.setValueAtTime(0.02, pulseAt)
    noiseGain.gain.linearRampToValueAtTime(0.13, pulseAt + 0.002)
    noiseGain.gain.exponentialRampToValueAtTime(0.02, pulseAt + 0.026)
  }

  noise.connect(noiseHighPass)
  noiseHighPass.connect(noiseLowPass)
  noiseLowPass.connect(noiseGain)
  noiseGain.connect(master)
  noise.start(startAt)
  noise.stop(startAt + feedDuration)
  sources.push(noise)

  const motor = context.createOscillator()
  const motorGain = context.createGain()
  motor.type = 'sawtooth'
  motor.frequency.setValueAtTime(92, startAt)
  motor.frequency.linearRampToValueAtTime(116, startAt + 0.28)
  motor.frequency.setValueAtTime(116, startAt + feedDuration - 0.2)
  motor.frequency.exponentialRampToValueAtTime(62, startAt + feedDuration)
  motorGain.gain.setValueAtTime(0.04, startAt)
  motorGain.gain.exponentialRampToValueAtTime(0.0001, startAt + feedDuration)
  motor.connect(motorGain)
  motorGain.connect(master)
  motor.start(startAt)
  motor.stop(startAt + feedDuration)
  sources.push(motor)

  const headWhine = context.createOscillator()
  const headGain = context.createGain()
  headWhine.type = 'square'
  headWhine.frequency.setValueAtTime(360, startAt)
  headWhine.frequency.linearRampToValueAtTime(430, startAt + 0.28)
  headWhine.frequency.setValueAtTime(430, startAt + feedDuration)
  headGain.gain.setValueAtTime(0.0055, startAt)
  headGain.gain.exponentialRampToValueAtTime(0.0001, startAt + feedDuration)
  headWhine.connect(headGain)
  headGain.connect(master)
  headWhine.start(startAt)
  headWhine.stop(startAt + feedDuration)
  sources.push(headWhine)

  // Alternating type bars: low body hit plus a bright metal key strike. The
  // tiny timing variation keeps the five-second feed from sounding looped.
  let keyIndex = 0
  for (let offset = 0.035; offset < feedDuration - 0.06; offset += keyIndex % 4 === 3 ? 0.078 : 0.061) {
    const keyAt = startAt + offset
    const body = context.createOscillator()
    const bodyGain = context.createGain()
    body.type = 'triangle'
    body.frequency.setValueAtTime(keyIndex % 2 === 0 ? 235 : 270, keyAt)
    body.frequency.exponentialRampToValueAtTime(96, keyAt + 0.026)
    bodyGain.gain.setValueAtTime(0.115, keyAt)
    bodyGain.gain.exponentialRampToValueAtTime(0.0001, keyAt + 0.034)
    body.connect(bodyGain)
    bodyGain.connect(master)
    body.start(keyAt)
    body.stop(keyAt + 0.04)
    sources.push(body)

    const metal = context.createOscillator()
    const metalGain = context.createGain()
    metal.type = 'square'
    metal.frequency.setValueAtTime(keyIndex % 3 === 0 ? 1840 : 1560, keyAt)
    metalGain.gain.setValueAtTime(0.018, keyAt)
    metalGain.gain.exponentialRampToValueAtTime(0.0001, keyAt + 0.018)
    metal.connect(metalGain)
    metalGain.connect(master)
    metal.start(keyAt)
    metal.stop(keyAt + 0.022)
    sources.push(metal)
    keyIndex += 1
  }

  const buttonClick = context.createOscillator()
  const buttonClickGain = context.createGain()
  buttonClick.type = 'triangle'
  buttonClick.frequency.setValueAtTime(176, startAt)
  buttonClick.frequency.exponentialRampToValueAtTime(72, startAt + 0.045)
  buttonClickGain.gain.setValueAtTime(0.095, startAt)
  buttonClickGain.gain.exponentialRampToValueAtTime(0.0001, startAt + 0.06)
  buttonClick.connect(buttonClickGain)
  buttonClickGain.connect(master)
  buttonClick.start(startAt)
  buttonClick.stop(startAt + 0.065)
  sources.push(buttonClick)

  const cutterAt = startAt + feedDuration + cutterStartDelay
  for (const [offset, startFrequency] of [[0, 138], [0.16, 104]] as const) {
    const cutter = context.createOscillator()
    const cutterGain = context.createGain()
    const hitAt = cutterAt + offset
    cutter.type = 'triangle'
    cutter.frequency.setValueAtTime(startFrequency, hitAt)
    cutter.frequency.exponentialRampToValueAtTime(42, hitAt + 0.055)
    cutterGain.gain.setValueAtTime(0.19, hitAt)
    cutterGain.gain.exponentialRampToValueAtTime(0.0001, hitAt + 0.075)
    cutter.connect(cutterGain)
    cutterGain.connect(master)
    cutter.start(hitAt)
    cutter.stop(hitAt + 0.08)
    sources.push(cutter)
  }

  // End with a physical machine action instead of a digital notification:
  // latch release, heavy carriage/lid close, then a short steel spring ring.
  for (const [offset, frequency, gainValue, duration] of [
    [0, 520, 0.32, 0.045],
    [0.075, 148, 0.62, 0.13],
    [0.115, 72, 0.52, 0.2]
  ] as const) {
    const closeHit = context.createOscillator()
    const closeGain = context.createGain()
    const hitAt = mechanicalCloseAt + offset
    closeHit.type = offset === 0 ? 'square' : 'triangle'
    closeHit.frequency.setValueAtTime(frequency, hitAt)
    closeHit.frequency.exponentialRampToValueAtTime(Math.max(36, frequency * 0.42), hitAt + duration)
    closeGain.gain.setValueAtTime(gainValue, hitAt)
    closeGain.gain.exponentialRampToValueAtTime(0.0001, hitAt + duration)
    closeHit.connect(closeGain)
    closeGain.connect(completionCompressor)
    closeHit.start(hitAt)
    closeHit.stop(hitAt + duration + 0.02)
    sources.push(closeHit)
  }

  const closeNoiseDuration = 0.2
  const closeNoiseBuffer = context.createBuffer(
    1,
    Math.ceil(context.sampleRate * closeNoiseDuration),
    context.sampleRate
  )
  const closeNoiseData = closeNoiseBuffer.getChannelData(0)
  for (let index = 0; index < closeNoiseData.length; index += 1) {
    const progress = index / closeNoiseData.length
    closeNoiseData[index] = ((Math.random() * 2) - 1) * ((1 - progress) ** 3)
  }
  const closeNoise = context.createBufferSource()
  const closeNoiseFilter = context.createBiquadFilter()
  const closeNoiseGain = context.createGain()
  closeNoise.buffer = closeNoiseBuffer
  closeNoiseFilter.type = 'bandpass'
  closeNoiseFilter.frequency.value = 980
  closeNoiseFilter.Q.value = 0.7
  closeNoiseGain.gain.setValueAtTime(0.48, mechanicalCloseAt + 0.07)
  closeNoiseGain.gain.exponentialRampToValueAtTime(0.0001, mechanicalCloseAt + 0.27)
  closeNoise.connect(closeNoiseFilter)
  closeNoiseFilter.connect(closeNoiseGain)
  closeNoiseGain.connect(completionCompressor)
  closeNoise.start(mechanicalCloseAt + 0.07)
  closeNoise.stop(mechanicalCloseAt + 0.29)
  sources.push(closeNoise)

  for (const [frequency, gainValue, duration] of [
    [610, 0.28, 0.52],
    [1220, 0.13, 0.36],
    [2440, 0.045, 0.23]
  ] as const) {
    const spring = context.createOscillator()
    const springGain = context.createGain()
    const springAt = mechanicalCloseAt + 0.12
    spring.type = 'sine'
    spring.frequency.setValueAtTime(frequency, springAt)
    spring.frequency.exponentialRampToValueAtTime(frequency * 0.965, springAt + duration)
    springGain.gain.setValueAtTime(gainValue, springAt)
    springGain.gain.exponentialRampToValueAtTime(0.0001, springAt + duration)
    spring.connect(springGain)
    springGain.connect(completionCompressor)
    spring.start(springAt)
    spring.stop(springAt + duration + 0.02)
    sources.push(spring)
  }

  void context.resume().catch(() => undefined)

  let stopped = false
  const timeoutId = window.setTimeout(
    () => stop(),
    PRINT_DURATION_MS + CUTTER_START_DELAY_MS + (mechanicalCloseDuration * 1000) + 500 + SOUND_CLEANUP_GRACE_MS
  )

  function stop(): void {
    if (stopped) return
    stopped = true
    window.clearTimeout(timeoutId)
    for (const source of sources) {
      try {
        source.stop()
      } catch {
        // A source that has already ended needs no further cleanup.
      }
    }
    void context.close().catch(() => undefined)
  }

  return stop
}
