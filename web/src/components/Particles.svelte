<script lang="ts">
  import { onMount, onDestroy } from 'svelte';
  import { currentMode } from '../stores/session';

  let canvas: HTMLCanvasElement;
  let animId: number;

  interface Particle {
    x: number; y: number;
    vx: number; vy: number;
    size: number; opacity: number;
    pulse: number;
  }

  const PARTICLE_COUNT = 35;

  let colorR = 0, colorG = 220, colorB = 255;
  let targetR = 0, targetG = 220, targetB = 255;
  let speedMultiplier = 1;
  let targetSpeed = 1;

  const unsubscribe = currentMode.subscribe(mode => {
    if (mode === 'otr') {
      targetR = 255; targetG = 171; targetB = 0;
      targetSpeed = 0.5;
    } else {
      targetR = 0; targetG = 220; targetB = 255;
      targetSpeed = 1;
    }
  });

  function lerp(a: number, b: number, t: number): number {
    return a + (b - a) * t;
  }

  onMount(() => {
    const ctx = canvas.getContext('2d');
    if (!ctx) return;

    const resize = () => {
      canvas.width = window.innerWidth;
      canvas.height = window.innerHeight;
    };
    resize();
    window.addEventListener('resize', resize);

    const particles: Particle[] = Array.from({ length: PARTICLE_COUNT }, () => ({
      x: Math.random() * canvas.width,
      y: Math.random() * canvas.height,
      vx: (Math.random() - 0.5) * 0.3,
      vy: (Math.random() - 0.5) * 0.3,
      size: Math.random() * 2 + 0.5,
      opacity: Math.random() * 0.2 + 0.05,
      pulse: Math.random() * Math.PI * 2,
    }));

    function draw() {
      if (!ctx) return;
      ctx.clearRect(0, 0, canvas.width, canvas.height);

      colorR = lerp(colorR, targetR, 0.02);
      colorG = lerp(colorG, targetG, 0.02);
      colorB = lerp(colorB, targetB, 0.02);
      speedMultiplier = lerp(speedMultiplier, targetSpeed, 0.02);

      for (const p of particles) {
        p.x += p.vx * speedMultiplier;
        p.y += p.vy * speedMultiplier;
        p.pulse += 0.008;

        if (p.x < 0) p.x = canvas.width;
        if (p.x > canvas.width) p.x = 0;
        if (p.y < 0) p.y = canvas.height;
        if (p.y > canvas.height) p.y = 0;

        const currentOpacity = p.opacity + Math.sin(p.pulse) * 0.06;
        const r = Math.round(colorR);
        const g = Math.round(colorG);
        const b = Math.round(colorB);

        ctx.beginPath();
        ctx.arc(p.x, p.y, p.size, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(${r}, ${g}, ${b}, ${Math.max(0, currentOpacity)})`;
        ctx.fill();

        ctx.beginPath();
        ctx.arc(p.x, p.y, p.size * 3, 0, Math.PI * 2);
        ctx.fillStyle = `rgba(${r}, ${g}, ${b}, ${Math.max(0, currentOpacity * 0.3)})`;
        ctx.fill();
      }

      animId = requestAnimationFrame(draw);
    }

    draw();

    return () => {
      window.removeEventListener('resize', resize);
    };
  });

  onDestroy(() => {
    if (animId) cancelAnimationFrame(animId);
    unsubscribe();
  });
</script>

<canvas bind:this={canvas} class="particles"></canvas>

<style>
  .particles {
    position: fixed;
    inset: 0;
    z-index: 0;
    pointer-events: none;
  }
</style>
