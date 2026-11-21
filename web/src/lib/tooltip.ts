// v-tip: M3 plain tooltip on mouse hover and keyboard focus, in place of the browser's
// title tooltip. One shared element, positioned above (or below) the target. Never on a touch
// screen: no tooltip from a tap (its focus), a touch's pointer, or on a device without hover; the
// aria-label it sets still names the button for screen readers.
import type { Directive } from 'vue';

let el: HTMLDivElement | null = null;
let timer: ReturnType<typeof setTimeout> | undefined;

function bubble(): HTMLDivElement {
    if (!el) {
        el = document.createElement('div');
        el.className = 'm3-tooltip';
        el.setAttribute('role', 'tooltip');
        document.body.append(el);
    }
    return el;
}

function show(target: HTMLElement) {
    const text = target.dataset.tip;
    if (!text) return;
    clearTimeout(timer);
    timer = setTimeout(() => {
        const b = bubble();
        b.textContent = text;
        b.classList.add('show');
        const r = target.getBoundingClientRect();
        const w = b.offsetWidth;
        const h = b.offsetHeight;
        const left = Math.min(Math.max(8, r.left + r.width / 2 - w / 2), window.innerWidth - w - 8);
        const top = r.top - h - 8 > 8 ? r.top - h - 8 : r.bottom + 8;
        b.style.left = `${left}px`;
        b.style.top = `${top}px`;
    }, 350);
}

function hide() {
    clearTimeout(timer);
    el?.classList.remove('show');
}

const canHover = () => typeof matchMedia !== 'function' || matchMedia('(hover: hover) and (pointer: fine)').matches;
// The last input was a touch: a focus that follows it is the tap's, not the keyboard's
let touched = false;
if (typeof window !== 'undefined') {
    window.addEventListener(
        'pointerdown',
        (e) => {
            touched = e.pointerType !== 'mouse';
            if (touched) hide();
        },
        { capture: true, passive: true }
    );
    window.addEventListener('keydown', () => (touched = false), { capture: true, passive: true });
}

export const vTip: Directive<HTMLElement, string | undefined> = {
    mounted(target, binding) {
        target.dataset.tip = binding.value ?? '';
        if (binding.value && !target.getAttribute('aria-label')) target.setAttribute('aria-label', binding.value);
        target.addEventListener('pointerenter', (e) => {
            if (e.pointerType === 'mouse' && canHover()) show(target);
        });
        target.addEventListener('pointerleave', hide);
        target.addEventListener('focus', () => {
            if (!touched && target.matches(':focus-visible')) show(target);
        });
        target.addEventListener('blur', hide);
    },
    updated(target, binding) {
        target.dataset.tip = binding.value ?? '';
    },
    beforeUnmount: hide,
};
