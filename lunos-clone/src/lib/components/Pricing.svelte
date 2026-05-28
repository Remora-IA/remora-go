<script>
  let activeTab = 'plataforma'
  let openFaq = null
  function toggleFaq(i) { openFaq = openFaq === i ? null : i }

  // Tabla comparativa por tab
  const tableRows = {
    plataforma: [
      { label: 'Orquestación declarativa', team: true, pro: true, enterprise: true, sub: [
        { label: 'Workflows definidos en archivo de configuración (sin código)', team: true, pro: true, enterprise: true },
        { label: 'Routing automático entre capabilities', team: true, pro: true, enterprise: true },
        { label: 'Retry logic y manejo de errores', team: true, pro: true, enterprise: true },
        { label: 'Workflows condicionales y branching avanzado', team: false, pro: true, enterprise: true },
        { label: 'Composición de sub-orquestadores', team: false, pro: true, enterprise: true },
      ]},
      { label: 'Frameworks / Capabilities', team: true, pro: true, enterprise: true, sub: [
        { label: 'Frameworks built-in (echo, foco, sabio, mensajero…)', team: true, pro: true, enterprise: true },
        { label: 'Deploy de frameworks personalizados', team: 'Hasta 3', pro: 'Ilimitados', enterprise: 'Ilimitados' },
        { label: 'Framework registry privado', team: false, pro: true, enterprise: true },
        { label: 'Versionado de frameworks', team: false, pro: true, enterprise: true },
      ]},
      { label: 'Modelos de IA compatibles', team: true, pro: true, enterprise: true, sub: [
        { label: 'Groq, Minimax', team: true, pro: true, enterprise: true },
        { label: 'OpenAI, Anthropic, otros proveedores', team: false, pro: true, enterprise: true },
        { label: 'Modelos locales / self-hosted', team: false, pro: false, enterprise: true },
        { label: 'Rotación automática de modelos por costo/latencia', team: false, pro: false, enterprise: true },
      ]},
      { label: 'Observabilidad', team: true, pro: true, enterprise: true, sub: [
        { label: 'Dashboard de ejecuciones en tiempo real', team: true, pro: true, enterprise: true },
        { label: 'Logs de decisiones de agentes', team: '7 días', pro: '90 días', enterprise: 'Ilimitado' },
        { label: 'Alertas y notificaciones', team: false, pro: true, enterprise: true },
        { label: 'Exportación de audit trails (compliance)', team: false, pro: false, enterprise: true },
      ]},
      { label: 'Seguridad', team: true, pro: true, enterprise: true, sub: [
        { label: 'Canales cifrados entre frameworks', team: true, pro: true, enterprise: true },
        { label: 'Vault para gestión de secrets', team: true, pro: true, enterprise: true },
        { label: 'SSO / autenticación empresarial', team: false, pro: false, enterprise: true },
        { label: 'SOC 2 Type 2', team: false, pro: false, enterprise: true },
        { label: 'Deploy en VPC privada', team: false, pro: false, enterprise: true },
      ]},
    ],
    integraciones: [
      { label: 'Canales de comunicación', team: true, pro: true, enterprise: true, sub: [
        { label: 'JSON-RPC sobre canal estándar', team: true, pro: true, enterprise: true },
        { label: 'Webhooks entrantes y salientes', team: true, pro: true, enterprise: true },
        { label: 'gRPC / streaming', team: false, pro: true, enterprise: true },
      ]},
      { label: 'Herramientas de desarrollo', team: true, pro: true, enterprise: true, sub: [
        { label: 'CLI de Remora', team: true, pro: true, enterprise: true },
        { label: 'SDK (Go)', team: true, pro: true, enterprise: true },
        { label: 'API REST de administración', team: false, pro: true, enterprise: true },
        { label: 'Terraform provider', team: false, pro: false, enterprise: true },
      ]},
      { label: 'Soporte', team: true, pro: true, enterprise: true, sub: [
        { label: 'Documentación y guías', team: true, pro: true, enterprise: true },
        { label: 'Soporte por email', team: false, pro: true, enterprise: true },
        { label: 'Canal privado de Slack', team: false, pro: false, enterprise: true },
        { label: 'SLA garantizado', team: false, pro: false, enterprise: true },
        { label: 'Onboarding dedicado', team: false, pro: false, enterprise: true },
      ]},
    ]
  }

  const faqs = [
    { q: '¿Cómo se cuenta el uso en el plan Gratis?', a: 'Cada ejecución de un workflow cuenta como una ejecución. Una ejecución incluye todos los pasos del workflow, sin importar cuántos frameworks participen. El plan Gratis incluye 1.000 ejecuciones por mes.' },
    { q: '¿Puedo usar mis propias claves de API de modelos de IA?', a: 'Sí. Remora es model-agnostic — usás tus propias claves de Groq, Minimax, OpenAI o cualquier proveedor compatible. Remora no cobra por tokens de modelo; solo por orquestación.' },
    { q: '¿Qué pasa si supero el límite de ejecuciones?', a: 'Te avisamos antes de llegar al límite. Podés upgrade en cualquier momento, o esperar al próximo ciclo de facturación. No cortamos workflows en el medio de una ejecución.' },
    { q: '¿Remora funciona on-premise?', a: 'El plan Enterprise permite deploy en infraestructura propia o VPC privada. Los planes Gratis y Pro corren sobre la infraestructura cloud de Remora.' },
    { q: '¿Qué tan rápido puedo tener mi primer workflow corriendo?', a: 'La mayoría de los equipos tienen su primer workflow en producción dentro de las primeras horas. La configuración declarativa significa que no escribís código de integración — solo definís qué debe pasar y Remora maneja el cómo.' },
    { q: '¿Remora reemplaza a mi equipo de ingeniería?', a: 'No. Remora elimina el trabajo repetitivo de plomería de agentes — routing, manejo de errores, composición de capabilities — pero los ingenieros siguen siendo esenciales para diseñar estrategias, construir frameworks personalizados y optimizar el rendimiento.' },
  ]

  const testimonials = [
    { quote: 'Pasamos de semanas de integración a tener nuevos workflows en producción en horas. La orquestación declarativa cambia completamente la forma de trabajar con agentes.', author: 'Equipo de plataforma IA', company: 'Startup fintech' },
    { quote: 'Por fin tenemos visibilidad real de lo que hacen nuestros agentes. El dashboard de observabilidad nos permite depurar en minutos lo que antes nos llevaba días.', author: 'Lead de ingeniería', company: 'Scale-up SaaS' },
    { quote: 'Lo que más nos gustó fue que no nos obliga a reescribir nada. Conecta con nuestros servicios existentes via JSON-RPC y ya están como capabilities del orquestador.', author: 'CTO', company: 'Agencia de automatización' },
  ]
</script>

<!-- Hero Section -->
<section style="background:#e8e9eb; padding:80px 0 0;">
  <div class="w-layout-blockcontainer container w-container" style="text-align:center; padding-top:40px; padding-bottom:60px;">
    <p style="font-size:13px; font-weight:600; color:#FE42A0; letter-spacing:.1em; text-transform:uppercase; margin:0 0 16px;">Planes y precios</p>
    <h1 style="font-size:clamp(40px,6vw,80px); font-weight:700; color:#030712; margin:0 0 20px; letter-spacing:-2px; line-height:1.05;">
      Orquestá agentes de IA.<br>Empezá sin pagar nada.
    </h1>
    <p style="font-size:18px; color:#6b7280; max-width:520px; margin:0 auto 32px;">
      Remora te cobra por lo que ejecutás, no por existir. Probá gratis y escalá cuando tengas resultados.
    </p>
    <div style="display:inline-flex; align-items:center; gap:8px; background:white; border:1px solid #e5e7eb; border-radius:100px; padding:4px 8px 4px 16px; font-size:15px; color:#374151;">
      <span>¿Tenés un volumen alto de ejecuciones?</span>
      <a href="/book-a-demo" style="background:#030712; color:white; padding:6px 14px; border-radius:100px; text-decoration:none; font-weight:500;">Hablemos →</a>
    </div>
  </div>

  <!-- Plans Cards -->
  <div class="w-layout-blockcontainer container w-container" style="padding-bottom:60px;">
    <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:20px; max-width:960px; margin:0 auto;">

      <!-- Gratis -->
      <div style="background:white; border-radius:20px; padding:28px; display:flex; flex-direction:column; gap:20px; border:1px solid #e5e7eb;">
        <div>
          <p style="font-size:12px; color:#9ca3af; font-weight:600; margin:0 0 8px; text-transform:uppercase; letter-spacing:.08em;">Gratis</p>
          <div style="display:flex; align-items:baseline; gap:4px;">
            <span style="font-size:40px; font-weight:700; color:#030712;">$0</span>
            <span style="font-size:16px; color:#9ca3af;">/mes</span>
          </div>
          <p style="font-size:15px; color:#6b7280; margin:8px 0 0; line-height:1.5;">Para explorar y hacer tus primeros prototipos.</p>
        </div>
        <ul style="list-style:none; padding:0; margin:0; display:flex; flex-direction:column; gap:12px; font-size:15px; color:#374151; flex:1;">
          {#each ['1.000 ejecuciones/mes','Hasta 3 frameworks propios','Frameworks built-in incluidos','Logs 7 días','Canales cifrados + Vault','Comunidad en Discord'] as item}
            <li style="display:flex; align-items:center; gap:10px;">
              <svg width="17" height="17" viewBox="0 0 20 20" fill="#030712" style="flex-shrink:0;"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              {item}
            </li>
          {/each}
        </ul>
        <a href="http://localhost:5181" style="display:block; text-align:center; border:2px solid #030712; color:#030712; font-weight:600; font-size:15px; padding:13px 20px; border-radius:8px; text-decoration:none;">
          Empezar gratis
        </a>
      </div>

      <!-- Pro — destacado -->
      <div style="background:#030712; border-radius:20px; padding:28px; display:flex; flex-direction:column; gap:20px; position:relative; overflow:hidden;">
        <div style="position:absolute; top:16px; right:16px; background:#FE42A0; color:white; font-size:11px; font-weight:700; padding:4px 12px; border-radius:6px; text-transform:uppercase; letter-spacing:.06em;">Más popular</div>
        <div>
          <p style="font-size:12px; color:#9ca3af; font-weight:600; margin:0 0 8px; text-transform:uppercase; letter-spacing:.08em;">Pro</p>
          <div style="display:flex; align-items:baseline; gap:4px;">
            <span style="font-size:40px; font-weight:700; color:white;">$79</span>
            <span style="font-size:16px; color:#9ca3af;">/mes</span>
          </div>
          <p style="font-size:15px; color:#9ca3af; margin:8px 0 0; line-height:1.5;">Para equipos que llevan agentes a producción.</p>
        </div>
        <ul style="list-style:none; padding:0; margin:0; display:flex; flex-direction:column; gap:12px; font-size:15px; color:#d1d5db; flex:1;">
          {#each ['50.000 ejecuciones/mes','Frameworks ilimitados','Todos los modelos de IA compatibles','Registry privado de frameworks','Logs 90 días + alertas','API REST de administración','Soporte por email (24h)'] as item}
            <li style="display:flex; align-items:center; gap:10px;">
              <svg width="17" height="17" viewBox="0 0 20 20" fill="#FE42A0" style="flex-shrink:0;"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              {item}
            </li>
          {/each}
        </ul>
        <a href="http://localhost:5181" style="display:block; text-align:center; background:#FE42A0; color:white; font-weight:600; font-size:15px; padding:13px 20px; border-radius:8px; text-decoration:none;">
          Empezar con Pro →
        </a>
      </div>

      <!-- Enterprise -->
      <div style="background:white; border-radius:20px; padding:28px; display:flex; flex-direction:column; gap:20px; border:1px solid #e5e7eb;">
        <div>
          <p style="font-size:12px; color:#9ca3af; font-weight:600; margin:0 0 8px; text-transform:uppercase; letter-spacing:.08em;">Enterprise</p>
          <div style="display:flex; align-items:baseline; gap:4px;">
            <span style="font-size:32px; font-weight:700; color:#030712;">A medida</span>
          </div>
          <p style="font-size:15px; color:#6b7280; margin:8px 0 0; line-height:1.5;">Para organizaciones con necesidades avanzadas.</p>
        </div>
        <ul style="list-style:none; padding:0; margin:0; display:flex; flex-direction:column; gap:12px; font-size:15px; color:#374151; flex:1;">
          {#each ['Ejecuciones ilimitadas','Deploy en VPC privada / on-premise','Modelos locales self-hosted','SSO + gestión de usuarios','Logs ilimitados + audit trails','SLA garantizado','Canal privado de Slack + onboarding'] as item}
            <li style="display:flex; align-items:center; gap:10px;">
              <svg width="17" height="17" viewBox="0 0 20 20" fill="#030712" style="flex-shrink:0;"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              {item}
            </li>
          {/each}
        </ul>
        <a href="/book-a-demo" style="display:block; text-align:center; border:2px solid #030712; color:#030712; font-weight:600; font-size:15px; padding:13px 20px; border-radius:8px; text-decoration:none;">
          Hablar con ventas →
        </a>
      </div>
    </div>
  </div>
</section>

<!-- Qué incluye Remora en todos los planes -->
<section style="background:#e8e9eb; padding:0 0 60px;">
  <div class="w-layout-blockcontainer container w-container">
    <div style="background:#1a2035; border-radius:24px; padding:48px; overflow:hidden;">
      <h2 style="font-size:28px; font-weight:700; color:white; margin:0 0 8px;">En todos los planes</h2>
      <p style="color:#9ca3af; font-size:15px; margin:0 0 36px;">El núcleo de Remora está siempre disponible, sin importar el plan.</p>

      <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:16px;">
        {#each [
          { icon: 'workflow', title: 'Orquestación declarativa', desc: 'Definí workflows en un archivo de configuración. Sin código de integración, sin pipelines frágiles.' },
          { icon: 'channel', title: 'Canal JSON-RPC', desc: 'Tus frameworks se comunican sobre un protocolo estándar. Cualquier servicio con API se convierte en capability.' },
          { icon: 'scale', title: 'Alta concurrencia (Go)', desc: 'Construido en Go para máxima performance. Corre miles de agentes en paralelo sin degradación.' },
          { icon: 'vault', title: 'Vault de secrets', desc: 'Gestión segura de claves y credenciales. Ningún secret viaja en texto plano por los canales.' },
          { icon: 'observe', title: 'Dashboard de observabilidad', desc: 'Mirá qué hace cada agente, en cada paso, en tiempo real. Depurá con contexto completo.' },
          { icon: 'model', title: 'Model-agnostic', desc: 'Groq, Minimax, y más. Cambiás de modelo en la config, sin tocar el código de tus frameworks.' },
        ] as feature}
          <div style="background:#0f1628; border-radius:14px; padding:22px;">
            <div style="margin-bottom:14px;">
              {#if feature.icon === 'workflow'}
                <svg width="24" height="24" fill="none" viewBox="0 0 24 24" stroke="#FE42A0" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M4 6h16M4 12h10M4 18h7"/></svg>
              {:else if feature.icon === 'channel'}
                <svg width="24" height="24" fill="none" viewBox="0 0 24 24" stroke="#FE42A0" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M8.288 15.038a5.25 5.25 0 017.424 0M5.106 11.856c3.807-3.808 9.98-3.808 13.788 0M1.924 8.674c5.565-5.565 14.587-5.565 20.152 0M12.53 18.22l-.53.53-.53-.53a.75.75 0 011.06 0z"/></svg>
              {:else if feature.icon === 'scale'}
                <svg width="24" height="24" fill="none" viewBox="0 0 24 24" stroke="#FE42A0" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3 13.125C3 12.504 3.504 12 4.125 12h2.25c.621 0 1.125.504 1.125 1.125v6.75C7.5 20.496 6.996 21 6.375 21h-2.25A1.125 1.125 0 013 19.875v-6.75zM9.75 8.625c0-.621.504-1.125 1.125-1.125h2.25c.621 0 1.125.504 1.125 1.125v11.25c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V8.625zM16.5 4.125c0-.621.504-1.125 1.125-1.125h2.25C20.496 3 21 3.504 21 4.125v15.75c0 .621-.504 1.125-1.125 1.125h-2.25a1.125 1.125 0 01-1.125-1.125V4.125z"/></svg>
              {:else if feature.icon === 'vault'}
                <svg width="24" height="24" fill="none" viewBox="0 0 24 24" stroke="#FE42A0" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M16.5 10.5V6.75a4.5 4.5 0 10-9 0v3.75m-.75 11.25h10.5a2.25 2.25 0 002.25-2.25v-6.75a2.25 2.25 0 00-2.25-2.25H6.75a2.25 2.25 0 00-2.25 2.25v6.75a2.25 2.25 0 002.25 2.25z"/></svg>
              {:else if feature.icon === 'observe'}
                <svg width="24" height="24" fill="none" viewBox="0 0 24 24" stroke="#FE42A0" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M3.75 3v11.25A2.25 2.25 0 006 16.5h2.25M3.75 3h-1.5m1.5 0h16.5m0 0h1.5m-1.5 0v11.25A2.25 2.25 0 0118 16.5h-2.25m-7.5 0h7.5m-7.5 0l-1 3m8.5-3l1 3m0 0l.5 1.5m-.5-1.5h-9.5m0 0l-.5 1.5"/></svg>
              {:else}
                <svg width="24" height="24" fill="none" viewBox="0 0 24 24" stroke="#FE42A0" stroke-width="1.5"><path stroke-linecap="round" stroke-linejoin="round" d="M9.75 3.104v5.714a2.25 2.25 0 01-.659 1.591L5 14.5M9.75 3.104c-.251.023-.501.05-.75.082m.75-.082a24.301 24.301 0 014.5 0m0 0v5.714c0 .597.237 1.17.659 1.591L19.8 15.3M14.25 3.104c.251.023.501.05.75.082M19.8 15.3l-1.57.393A9.065 9.065 0 0112 15a9.065 9.065 0 00-6.23-.693L5 14.5m14.8.8l1.402 1.402c1.232 1.232.65 3.318-1.067 3.611A48.309 48.309 0 0112 21c-2.773 0-5.491-.235-8.135-.687-1.718-.293-2.3-2.379-1.067-3.61L5 14.5"/></svg>
              {/if}
            </div>
            <p style="font-size:16px; font-weight:600; color:white; margin:0 0 8px;">{feature.title}</p>
            <p style="font-size:14px; color:#9ca3af; margin:0; line-height:1.6;">{feature.desc}</p>
          </div>
        {/each}
      </div>
    </div>
  </div>
</section>

<!-- Tabla comparativa completa -->
<section style="background:#0f1628; padding:60px 0;">
  <div class="w-layout-blockcontainer container w-container">
    <h2 style="font-size:28px; font-weight:700; color:white; margin:0 0 32px;">Comparación completa de planes</h2>

    <!-- Tabs -->
    <div style="display:flex; gap:4px; margin-bottom:32px; background:#1a2035; border-radius:10px; padding:4px; width:fit-content;">
      {#each [['plataforma','Plataforma'],['integraciones','Integraciones y soporte']] as [id,label]}
        <button on:click={() => activeTab = id}
          style="padding:10px 22px; border-radius:7px; border:none; cursor:pointer; font-size:15px; font-weight:500; transition:all .15s;
            background:{activeTab===id ? 'white' : 'transparent'};
            color:{activeTab===id ? '#030712' : '#9ca3af'};">
          {label}
        </button>
      {/each}
    </div>

    <!-- Sticky header -->
    <div style="display:grid; grid-template-columns:2.5fr 1fr 1fr 1fr; gap:0; margin-bottom:12px; padding:0 16px;">
      <div></div>
      {#each [['Gratis','$0/mes'],['Pro','$79/mes'],['Enterprise','A medida']] as [name, price]}
        <div style="text-align:center; padding:4px 8px;">
          <p style="color:white; font-weight:600; font-size:15px; margin:0;">{name}</p>
          <p style="color:#9ca3af; font-size:13px; margin:3px 0 0;">{price}</p>
        </div>
      {/each}
    </div>

    {#each tableRows[activeTab] as row}
      <div style="border:1px solid #1e2a45; border-radius:10px; overflow:hidden; margin-bottom:8px;">
        <div style="display:grid; grid-template-columns:2.5fr 1fr 1fr 1fr; background:#1a2035; padding:14px 16px; align-items:center;">
          <span style="color:white; font-weight:600; font-size:15px;">{row.label}</span>
          {#each [row.team, row.pro, row.enterprise] as val}
            <div style="text-align:center;">
              {#if val === true}
                <svg width="18" height="18" viewBox="0 0 20 20" fill="#FE42A0" style="margin:0 auto;"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
              {:else if val === false}
                <span style="color:#374151;">—</span>
              {:else}
                <span style="color:#9ca3af; font-size:13px; line-height:1.3;">{val}</span>
              {/if}
            </div>
          {/each}
        </div>
        {#if row.sub && row.sub.length > 0}
          {#each row.sub as sub}
            <div style="display:grid; grid-template-columns:2.5fr 1fr 1fr 1fr; padding:10px 16px 10px 32px; border-top:1px solid #1e2a45; align-items:center;">
              <span style="color:#9ca3af; font-size:12px;">{sub.label}</span>
              {#each [sub.team, sub.pro, sub.enterprise] as val}
                <div style="text-align:center;">
                  {#if val === true}
                    <svg width="15" height="15" viewBox="0 0 20 20" fill="#FE42A0" style="margin:0 auto;"><path fill-rule="evenodd" d="M10 18a8 8 0 100-16 8 8 0 000 16zm3.707-9.293a1 1 0 00-1.414-1.414L9 10.586 7.707 9.293a1 1 0 00-1.414 1.414l2 2a1 1 0 001.414 0l4-4z" clip-rule="evenodd"/></svg>
                  {:else if val === false}
                    <span style="color:#374151; font-size:12px;">—</span>
                  {:else}
                    <span style="color:#9ca3af; font-size:10px; line-height:1.3;">{val}</span>
                  {/if}
                </div>
              {/each}
            </div>
          {/each}
        {/if}
      </div>
    {/each}
  </div>
</section>

<!-- Testimonios -->
<section style="background:#e8e9eb; padding:80px 0;">
  <div class="w-layout-blockcontainer container w-container">
    <p style="font-size:12px; color:#FE42A0; font-weight:600; text-transform:uppercase; letter-spacing:.1em; text-align:center; margin-bottom:8px;">Lo que dicen los equipos que ya usan Remora</p>
    <h2 style="font-size:32px; font-weight:700; color:#030712; text-align:center; margin:0 0 48px; max-width:600px; margin-left:auto; margin-right:auto;">Resultados reales desde el primer workflow</h2>
    <div style="display:grid; grid-template-columns:1fr 1fr 1fr; gap:24px;">
      {#each testimonials as t}
        <div style="background:white; border-radius:16px; padding:28px; display:flex; flex-direction:column; gap:16px; border:1px solid #e5e7eb;">
          <svg width="24" height="18" viewBox="0 0 24 18" fill="#FE42A0" opacity=".3"><path d="M0 18V10.8C0 7.6 .867 5.033 2.6 3.1 4.333 1.167 6.8.133 10 0l.8 1.6C8.667 2 7.167 2.8 6 4c-1.167 1.2-1.8 2.533-1.9 4H6V18H0zm14 0V10.8c0-3.2.867-5.767 2.6-7.7C18.333 1.167 20.8.133 24 0l.8 1.6c-2.133.4-3.633 1.2-4.8 2.4-1.167 1.2-1.8 2.533-1.9 4H20V18h-6z"/></svg>
          <p style="font-size:14px; color:#374151; line-height:1.7; margin:0; font-style:italic;">"{t.quote}"</p>
          <div style="border-top:1px solid #f3f4f6; padding-top:14px;">
            <p style="font-weight:600; color:#030712; font-size:13px; margin:0;">{t.author}</p>
            <p style="color:#9ca3af; font-size:12px; margin:3px 0 0;">{t.company}</p>
          </div>
        </div>
      {/each}
    </div>
  </div>
</section>

<!-- FAQ -->
<section style="background:#e8e9eb; padding:0 0 80px;">
  <div class="w-layout-blockcontainer container w-container" style="max-width:720px; margin:0 auto;">
    <h2 style="font-size:28px; font-weight:700; color:#030712; margin:0 0 32px;">Preguntas frecuentes</h2>
    {#each faqs as faq, i}
      <div style="border-top:1px solid #d1d5db; padding:18px 0;">
        <button on:click={() => toggleFaq(i)}
          style="width:100%; text-align:left; background:none; border:none; cursor:pointer; display:flex; justify-content:space-between; align-items:center; gap:16px; padding:0;">
          <span style="font-size:15px; font-weight:500; color:#030712;">{faq.q}</span>
          <svg width="18" height="18" fill="none" viewBox="0 0 24 24" stroke="#6b7280" stroke-width="2"
            style="flex-shrink:0; transform:{openFaq===i ? 'rotate(180deg)' : 'none'}; transition:transform .2s;">
            <path stroke-linecap="round" stroke-linejoin="round" d="M19 9l-7 7-7-7"/>
          </svg>
        </button>
        {#if openFaq === i}
          <p style="color:#6b7280; font-size:14px; margin:12px 0 0; line-height:1.6;">{faq.a}</p>
        {/if}
      </div>
    {/each}
    <div style="border-top:1px solid #d1d5db;"></div>
  </div>
</section>

<!-- CTA final -->
<section style="background:#030712; padding:80px 0;">
  <div class="w-layout-blockcontainer container w-container" style="text-align:center;">
    <h2 style="font-size:clamp(28px,4vw,52px); font-weight:700; color:white; margin:0 0 16px; line-height:1.1;">
      Tu primer workflow en producción<br>en horas, no en meses.
    </h2>
    <p style="color:#9ca3af; font-size:16px; margin:0 0 36px; max-width:480px; margin-left:auto; margin-right:auto;">
      Empezá gratis. Sin tarjeta de crédito. Sin compromisos.
    </p>
    <div style="display:flex; gap:12px; justify-content:center; flex-wrap:wrap;">
      <a href="http://localhost:5181"
        style="display:inline-flex; align-items:center; gap:8px; background:#FE42A0; color:white; font-weight:600; font-size:15px; padding:14px 28px; border-radius:8px; text-decoration:none;">
        Probar ya, gratis
        <svg width="14" height="14" fill="none" viewBox="0 0 16 17"><path d="M0.5 8.5H15.5" stroke="currentColor" stroke-linecap="round"/><path d="M10.5 3.5L15.5 8.5L10.5 13.5" stroke="currentColor" stroke-linecap="round" stroke-linejoin="round"/></svg>
      </a>
      <a href="/book-a-demo"
        style="display:inline-flex; align-items:center; gap:8px; border:2px solid #374151; color:white; font-weight:600; font-size:15px; padding:12px 28px; border-radius:8px; text-decoration:none;">
        Hablar con el equipo
      </a>
    </div>
  </div>
</section>
