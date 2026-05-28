<script>
  let companyName = ''
  let primaryColor = '#0B122A'
  let accentColor = '#FE42A0'
  let logoFile = null; let logoPreview = null; let logoError = ''

  function handleLogoChange(e) {
    const file = e.target.files?.[0]
    if (!file) return
    if (!file.type.startsWith('image/')) { logoError = 'PNG, JPG o SVG requerido.'; return }
    logoError = ''; logoFile = file
    const reader = new FileReader()
    reader.onload = ev => { logoPreview = ev.target.result }
    reader.readAsDataURL(file)
  }
  function removeLogo() { logoFile = null; logoPreview = null }
</script>

<div class="flex flex-col gap-4 max-w-4xl">
  <p class="text-sm text-gray-500">Conecta Remora a tus sistemas</p>
  <h1 class="text-2xl font-semibold text-gray-900 mb-4">Personaliza tu marca</h1>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-1">Logo de la empresa</p>
    <p class="text-xs text-gray-400 mb-4">Aparece en reportes, emails y el portal de clientes. PNG, SVG o JPG.</p>
    {#if logoPreview}
      <div class="flex items-center gap-4">
        <div class="w-20 h-20 rounded-xl border border-gray-200 flex items-center justify-center bg-gray-50 overflow-hidden">
          <img src={logoPreview} alt="preview" class="max-w-full max-h-full object-contain p-2" />
        </div>
        <div>
          <p class="text-sm font-medium text-gray-800">{logoFile?.name}</p>
          <button type="button" class="text-xs text-red-400 hover:text-red-600" on:click={removeLogo}>Eliminar</button>
        </div>
      </div>
    {:else}
      <label class="flex flex-col items-center justify-center w-full h-36 border-2 border-dashed border-gray-200 rounded-xl cursor-pointer hover:border-gray-300 hover:bg-gray-50 transition-colors">
        <svg class="w-8 h-8 text-gray-300 mb-2" fill="none" viewBox="0 0 24 24" stroke="currentColor">
          <path stroke-linecap="round" stroke-linejoin="round" stroke-width="1.5" d="M4 16l4.586-4.586a2 2 0 012.828 0L16 16m-2-2l1.586-1.586a2 2 0 012.828 0L20 14m-6-6h.01M6 20h12a2 2 0 002-2V6a2 2 0 00-2-2H6a2 2 0 00-2 2v12a2 2 0 002 2z"/>
        </svg>
        <p class="text-sm text-gray-400">Haz clic para subir o arrastra y suelta</p>
        <p class="text-xs text-gray-300 mt-1">SVG, PNG, JPG</p>
        <input type="file" accept="image/*" class="hidden" on:change={handleLogoChange} />
      </label>
      {#if logoError}<p class="text-xs text-red-500 mt-2">{logoError}</p>{/if}
    {/if}
  </div>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-1">Nombre de la empresa</p>
    <p class="text-xs text-gray-400 mb-4">Aparece en reportes y comunicaciones al cliente.</p>
    <input bind:value={companyName} type="text" placeholder="ej. Acme Corp"
      class="w-full max-w-md border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#FE42A0] placeholder:text-gray-300" />
  </div>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-1">Colores de marca</p>
    <p class="text-xs text-gray-400 mb-4">Usados en cabeceras de reportes, botones de email y el portal.</p>
    <div class="flex flex-col gap-4 max-w-sm">
      <div class="flex items-center gap-4">
        <label class="w-32 text-xs font-medium text-gray-600 flex-shrink-0">Primario</label>
        <div class="flex items-center gap-2 flex-1">
          <input type="color" bind:value={primaryColor} class="w-10 h-10 rounded-lg border border-gray-200 cursor-pointer p-0.5 bg-white" />
          <input type="text" bind:value={primaryColor} class="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-[#FE42A0]" />
        </div>
      </div>
      <div class="flex items-center gap-4">
        <label class="w-32 text-xs font-medium text-gray-600 flex-shrink-0">Acento</label>
        <div class="flex items-center gap-2 flex-1">
          <input type="color" bind:value={accentColor} class="w-10 h-10 rounded-lg border border-gray-200 cursor-pointer p-0.5 bg-white" />
          <input type="text" bind:value={accentColor} class="flex-1 border border-gray-200 rounded-lg px-3 py-2 text-sm font-mono focus:outline-none focus:ring-2 focus:ring-[#FE42A0]" />
        </div>
      </div>
    </div>
  </div>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-4">Vista previa</p>
    <div class="rounded-xl border border-gray-100 overflow-hidden shadow-sm text-sm">
      <div class="px-6 py-4 flex items-center justify-between" style="background-color:{primaryColor}">
        <div class="flex items-center gap-3">
          {#if logoPreview}
            <img src={logoPreview} alt="logo" class="h-8 w-auto object-contain rounded" />
          {:else}
            <div class="w-8 h-8 rounded-md bg-white/20 flex items-center justify-center text-white font-bold text-xs">
              {companyName?.[0]?.toUpperCase() || 'A'}
            </div>
          {/if}
          <span class="text-white font-semibold">{companyName || 'Tu Empresa'}</span>
        </div>
        <span class="text-white/70 text-xs">INVOICE #001</span>
      </div>
      <div class="px-6 py-4 bg-white">
        <div class="flex justify-between mb-3">
          <div><p class="text-gray-400 text-xs">Cliente</p><p class="font-medium text-gray-800">Acme Customer</p></div>
          <div class="text-right"><p class="text-gray-400 text-xs">Fecha de vencimiento</p><p class="font-medium text-gray-800">15 Jun, 2026</p></div>
        </div>
        <div class="border-t border-gray-100 pt-3 flex justify-between items-center">
          <span class="text-gray-500">Servicios profesionales</span>
          <span class="font-semibold text-gray-900">$4,800.00</span>
        </div>
        <div class="mt-4 flex justify-end">
          <button class="px-4 py-2 rounded-lg text-white text-xs font-medium" style="background-color:{accentColor}" type="button">Pagar ahora</button>
        </div>
      </div>
    </div>
  </div>

  <div class="flex justify-end pt-4 pb-8">
    <button class="bg-[#0B122A] text-white px-6 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors" type="button">Continuar</button>
  </div>
</div>
