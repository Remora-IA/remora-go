<script>
  import { onMount } from 'svelte'

  const words = ['construyendo', 'rediseñando', 'reimaginando']
  let wordIdx = 0
  let currentWord = words[0]
  let fading = false

  onMount(() => {
    const interval = setInterval(() => {
      fading = true
      setTimeout(() => {
        wordIdx = (wordIdx + 1) % words.length
        currentWord = words[wordIdx]
        fading = false
      }, 300)
    }, 2400)
    return () => clearInterval(interval)
  })

  const team = [
    { name: 'Tomás Bravo',   role: 'Founder & CEO',      initials: 'TB', color: '#7c3aed', li: 'https://linkedin.com' },
    { name: 'Rol disponible', role: 'Head of Engineering', initials: null },
    { name: 'Rol disponible', role: 'Head of Product',     initials: null },
    { name: 'Rol disponible', role: 'Head of Growth',      initials: null },
    { name: 'Rol disponible', role: 'Sr. Go Engineer',     initials: null },
    { name: 'Rol disponible', role: 'Founding Engineer',   initials: null },
    { name: 'Rol disponible', role: 'Founding Engineer',   initials: null },
    { name: 'Rol disponible', role: 'Product Designer',    initials: null },
  ]

  const values = [
    { name: 'Declarativo',       desc: 'Creemos que la complejidad de los sistemas no debería trasladarse a los ingenieros. Definís qué tiene que pasar, Remora se ocupa del cómo.' },
    { name: 'Observable',        desc: 'Un agente que no podés entender es un agente que no podés confiar. Visibilidad total en cada decisión, cada paso, cada workflow.' },
    { name: 'Sin vendor lock-in', desc: 'JSON-RPC estándar. Cualquier modelo, cualquier servicio, cualquier infraestructura. Remora se integra con tu stack, no al revés.' },
  ]
</script>

<!-- ─── HERO ─────────────────────────────────────────────────── -->
<section style="background:#e8e9eb; padding:120px 0 160px;">
  <div class="w-layout-blockcontainer container w-container">
    <h1 style="font-size:clamp(56px,8vw,96px); font-weight:700; color:#030712; line-height:1.05; letter-spacing:-3px; margin:0;">
      Estamos
      <span style="color:#9ca3af; opacity:{fading ? 0 : 1}; transition:opacity .3s ease;"> {currentWord}</span>
    </h1>
  </div>
</section>

<!-- ─── CARTA DEL FOUNDER ─────────────────────────────────────── -->
<section style="background:#e8e9eb; padding:0 0 80px;">
  <div class="w-layout-blockcontainer container w-container">
    <div style="background:white; border-radius:24px; padding:64px; display:grid; grid-template-columns:1fr 1.2fr; gap:80px; align-items:start;">

      <!-- Izquierda: taglines grises grandes -->
      <div style="display:flex; flex-direction:column; gap:32px;">
        <p style="font-size:clamp(20px,2.2vw,26px); color:#9ca3af; line-height:1.35; margin:0; font-weight:500;">
          Construir con agentes de IA es increíblemente poderoso. Y un caos total.
        </p>
        <p style="font-size:clamp(18px,2vw,24px); color:#9ca3af; line-height:1.35; margin:0; font-weight:500;">
          Es un wild west ahí afuera.
        </p>
        <p style="font-size:clamp(18px,2vw,24px); color:#6b7280; line-height:1.35; margin:0; font-weight:500;">
          Remora es la capa de orquestación que finalmente pone orden.
        </p>
      </div>

      <!-- Derecha: carta del founder -->
      <div style="font-size:16px; color:#374151; line-height:1.8; display:flex; flex-direction:column; gap:18px;">
        <p style="margin:0; font-style:italic; color:#6b7280;">Cuándo empecé a construir con agentes de IA, me topé con el mismo muro una y otra vez.</p>

        <p style="margin:0;">Cada integración nueva era código a mano. Cada pipeline era frágil. Cuando algo fallaba no había forma de saber por qué — ni qué agente tomó qué decisión, ni en qué paso del flujo se rompió todo. Para mover un modelo había que tocar código en cinco lugares.</p>

        <p style="margin:0;">Me di cuenta de que el problema no era la IA en sí. El problema era la <em>infraestructura alrededor</em>. La plomería. El routing. El manejo de errores. La observabilidad. Todo eso que no aporta valor de negocio pero consume el 80% del tiempo.</p>

        <p style="margin:0;">En vez de decirte cómo hacerlo diferente, construimos la plataforma que lo hace por vos. Remora orquesta tus agentes de forma declarativa — definís qué, nosotros nos ocupamos del cómo. Con visibilidad completa en cada paso, sin lock-in, conectando con lo que ya tenés.</p>

        <p style="margin:0;">Si eso suena como un problema que vale la pena resolver, bienvenido a bordo.</p>

        <div style="padding-top:8px;">
          <p style="font-weight:700; color:#030712; font-size:15px; margin:0;">Tomás Bravo</p>
          <p style="color:#9ca3af; font-size:14px; margin:4px 0 0;">Founder & CEO</p>
        </div>
      </div>
    </div>
  </div>
</section>

<!-- ─── STATS ─────────────────────────────────────────────────── -->
<section style="background:#e8e9eb; padding:0 0 80px;">
  <div class="w-layout-blockcontainer container w-container">
    <div style="display:grid; grid-template-columns:repeat(4,1fr); gap:16px;">
      {#each [
        { val: '100%',  label: 'Open en protocolo de comunicación',       bg: '#525c6e' },
        { val: 'Go',    label: 'Alta concurrencia nativa',                bg: '#6b7585' },
        { val: '< 1h',  label: 'Hasta el primer workflow en producción',  bg: '#848d9b' },
        { val: '∞',     label: 'Capabilities combinables sin reescribir', bg: '#9da5b1' },
      ] as s}
        <div style="background:{s.bg}; border-radius:18px; padding:32px 24px;">
          <p style="font-size:clamp(36px,4vw,52px); font-weight:700; color:white; margin:0 0 14px; line-height:1;">{s.val}</p>
          <p style="font-size:14px; color:rgba(255,255,255,.7); margin:0; line-height:1.5;">{s.label}</p>
        </div>
      {/each}
    </div>
  </div>
</section>

<!-- ─── EQUIPO ────────────────────────────────────────────────── -->
<section style="background:#e8e9eb; padding:0 0 80px;">
  <div class="w-layout-blockcontainer container w-container">
    <h2 style="font-size:clamp(44px,6vw,80px); font-weight:700; color:#030712; margin:0 0 48px; letter-spacing:-2px; line-height:1.05;">
      Conocé <span style="color:#9ca3af;">al</span> <span style="color:#c4c9d1;">equipo</span>
    </h2>
    <div style="display:grid; grid-template-columns:repeat(4,1fr); gap:16px;">
      {#each team as m}
        <div style="background:white; border-radius:18px; padding:24px; border:1px solid #e5e7eb;">
          <!-- Avatar -->
          <div style="width:72px; height:72px; border-radius:14px; margin-bottom:16px; overflow:hidden;
            background:{m.initials ? m.color : '#f3f4f6'};
            display:flex; align-items:center; justify-content:center;">
            {#if m.initials}
              <span style="color:white; font-size:24px; font-weight:700;">{m.initials}</span>
            {:else}
              <svg width="32" height="32" fill="none" viewBox="0 0 24 24" stroke="#d1d5db" stroke-width="1.5">
                <path stroke-linecap="round" stroke-linejoin="round" d="M15.75 6a3.75 3.75 0 11-7.5 0 3.75 3.75 0 017.5 0zM4.501 20.118a7.5 7.5 0 0114.998 0A17.933 17.933 0 0112 21.75c-2.676 0-5.216-.584-7.499-1.632z"/>
              </svg>
            {/if}
          </div>
          <p style="font-size:16px; font-weight:700; color:{m.initials ? '#030712' : '#9ca3af'}; margin:0 0 4px;">{m.name}</p>
          <p style="font-size:13px; color:#9ca3af; margin:0 0 14px;">{m.role}</p>
          {#if m.li}
            <a href={m.li} target="_blank" style="display:inline-flex; align-items:center; justify-content:center; width:28px; height:28px; background:#0a66c2; border-radius:5px; text-decoration:none;">
              <svg width="14" height="14" viewBox="0 0 24 24" fill="white"><path d="M16 8a6 6 0 016 6v7h-4v-7a2 2 0 00-2-2 2 2 0 00-2 2v7h-4v-7a6 6 0 016-6zM2 9h4v12H2z"/><circle cx="4" cy="4" r="2"/></svg>
            </a>
          {:else}
            <span style="font-size:11px; color:#d1d5db; font-style:italic;">Rol abierto</span>
          {/if}
        </div>
      {/each}
    </div>
  </div>
</section>

<!-- ─── VALORES ───────────────────────────────────────────────── -->
<section style="background:#e8e9eb; padding:0 0 80px;">
  <div class="w-layout-blockcontainer container w-container">
    <p style="font-size:13px; font-weight:600; color:#9ca3af; letter-spacing:.08em; text-transform:uppercase; margin:0 0 48px;">Nuestros valores</p>
    <div style="display:grid; grid-template-columns:repeat(3,1fr); gap:48px;">
      {#each values as v}
        <div style="border-top:1px solid #d1d5db; padding-top:28px;">
          <p style="font-size:20px; font-weight:700; color:#030712; margin:0 0 14px;">{v.name}</p>
          <p style="font-size:15px; color:#6b7280; margin:0; line-height:1.7;">{v.desc}</p>
        </div>
      {/each}
    </div>
  </div>
</section>

<!-- ─── CTA HIRING ────────────────────────────────────────────── -->
<section style="background:#e8e9eb; padding:0 0 120px;">
  <div class="w-layout-blockcontainer container w-container">
    <div style="display:grid; grid-template-columns:1fr 1fr; gap:64px; align-items:start;">
      <div>
        <h2 style="font-size:clamp(28px,3.5vw,44px); font-weight:700; color:#030712; margin:0 0 20px; line-height:1.15; letter-spacing:-1px;">
          Un equipo pequeño y ambicioso empujándose a dar lo mejor.
        </h2>
        <p style="font-size:16px; color:#6b7280; margin:0 0 32px; line-height:1.7;">
          Venís a construir infraestructura que otros equipos van a usar todos los días. Si te apasiona la IA y los sistemas distribuidos, hablemos.
        </p>
        <a href="mailto:hello@remora.dev" class="button w-inline-block" style="text-decoration:none; display:inline-flex; align-items:center; gap:8px;">
          <div>Sumate a Remora</div>
          <svg class="button-arrow" fill="none" viewBox="0 0 16 17" width="14"><path d="M0.5 8.5H15.5" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round"/><path d="M10.5 3.5L15.5 8.5L10.5 13.5" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round"/></svg>
        </a>
      </div>

      <div style="display:flex; flex-direction:column; gap:10px;">
        {#each [
          'Head of Engineering',
          'Senior Go Engineer',
          'Head of Growth',
          'Founding Account Executive',
        ] as job}
          <div style="background:white; border-radius:12px; padding:18px 22px; display:flex; justify-content:space-between; align-items:center; border:1px solid #e5e7eb;">
            <div>
              <p style="font-weight:600; color:#030712; font-size:15px; margin:0;">{job}</p>
              <p style="color:#9ca3af; font-size:13px; margin:4px 0 0;">Full-time · Remoto</p>
            </div>
            <svg width="16" height="16" fill="none" viewBox="0 0 16 17" stroke="#9ca3af" stroke-width="1.5">
              <path d="M0.5 8.5H15.5" stroke-linecap="round"/>
              <path d="M10.5 3.5L15.5 8.5L10.5 13.5" stroke-linecap="round" stroke-linejoin="round"/>
            </svg>
          </div>
        {/each}
      </div>
    </div>
  </div>
</section>
