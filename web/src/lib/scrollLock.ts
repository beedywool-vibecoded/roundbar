// Locks page scrolling while any modal is open (ref-counted for nested dialogs), keeping
// the scrollbar gutter to avoid a jump.
let count = 0

export function lockScroll() {
  if (count++ === 0) {
    const root = document.documentElement
    root.style.scrollbarGutter = 'stable'
    root.style.overflow = 'hidden'
  }
}

export function unlockScroll() {
  if (count > 0 && --count === 0) {
    const root = document.documentElement
    root.style.overflow = ''
    root.style.scrollbarGutter = ''
  }
}
