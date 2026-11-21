// v-tip: M3 plain tooltip on mouse hover and keyboard focus (never on touch), replacing title tooltips. One shared element.
import type { Directive } from 'vue'

let el: HTMLDivElement | null = null
let timer: ReturnType<typeof setTimeout> | undefined

function bubble(): HTMLDivElement {
  if (!el) {
    el = document.createElement('div')
    el.className = 'm3-tooltip'
    el.setAttribute('role', 'tooltip')
    document.body.append(el)
  }
  return el
}

function show(target: HTMLElement) {
  const text = target.dataset.tip
  if (!text) return
  clearTimeout(timer)
  timer = setTimeout(() => {
    const b = bubble()
    b.textContent = text
    b.classList.add('show')
    const r = target.getBoundingClientRect()
    const w = b.offsetWidth
    const h = b.offsetHeight
    const left = Math.min(Math.max(8, r.left + r.width / 2 - w / 2), window.innerWidth - w - 8)
    const top = r.top - h - 8 > 8 ? r.top - h - 8 : r.bottom + 8
    b.style.left = `${left}px`
    b.style.top = `${top}px`
  }, 350)
}

function hide() {
  clearTimeout(timer)
  el?.classList.remove('show')
}

export const vTip: Directive<HTMLElement, string | undefined> = {
  mounted(target, binding) {
    target.dataset.tip = binding.value ?? ''
    if (binding.value && !target.getAttribute('aria-label')) target.setAttribute('aria-label', binding.value)
    // Mouse hover and keyboard focus only: on a phone a tap is both a "hover" and a focus, and a
    // tooltip popping up after every tap only gets in the way (the label is read out anyway)
    target.addEventListener('pointerenter', (e) => e.pointerType === 'mouse' && show(target))
    target.addEventListener('pointerleave', hide)
    target.addEventListener('pointerdown', (e) => e.pointerType !== 'mouse' && hide())
    target.addEventListener('focus', () => target.matches(':focus-visible') && show(target))
    target.addEventListener('blur', hide)
  },
  updated(target, binding) {
    target.dataset.tip = binding.value ?? ''
  },
  beforeUnmount: hide,
}
