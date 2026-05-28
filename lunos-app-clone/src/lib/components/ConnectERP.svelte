<script>
  import IntegrationCard from './IntegrationCard.svelte'
  let selectedERP = null
  let apiKey = ''
  let connecting = false
  let connected = false

  const erpOptions = [
    { id: 'quickbooks', logo: '/logos/quickbooks.svg', alt: 'QuickBooks' },
    { id: 'xero',       logo: '/logos/xero.svg',       alt: 'Xero' },
    { id: 'netsuite',   logo: '/logos/netsuite.svg',   alt: 'NetSuite' },
    { id: 'sage',       logo: '/logos/sage.svg',       alt: 'Sage' },
    { id: 'zoho',       logo: '/logos/zoho.svg',       alt: 'Zoho' },
    { id: 'maxio',      logo: '/logos/maxio.svg',      alt: 'Maxio' },
    { id: 'chargebee',  logo: '/logos/chargebee.svg',  alt: 'Chargebee' },
    { id: 'zuora',      logo: '/logos/zuora.svg',      alt: 'Zuora' },
  ]
  const erpLabels = { quickbooks:'QuickBooks', xero:'Xero', netsuite:'NetSuite', sage:'Sage', zoho:'Zoho', maxio:'Maxio', chargebee:'Chargebee', zuora:'Zuora' }

  async function handleConnect() {
    connecting = true
    await new Promise(r => setTimeout(r, 1200))
    connecting = false; connected = true
  }
</script>

<div class="flex flex-col gap-4 max-w-4xl">
  <p class="text-sm text-gray-500">Conecta Remora a tus sistemas</p>
  <h1 class="text-2xl font-semibold text-gray-900 mb-4">Conecta tus fuentes de datos</h1>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-4">¿Qué plataforma quieres conectar primero?</p>
    <div class="grid grid-cols-4 gap-3">
      {#each erpOptions as item}
        <IntegrationCard {...item} selected={selectedERP === item.id}
          onClick={() => { selectedERP = item.id; connected = false }} />
      {/each}
    </div>
  </div>

  {#if selectedERP}
    <div class="border border-gray-200 rounded-xl p-6 bg-white">
      <p class="text-sm font-medium text-gray-800 mb-1">Conectar {erpLabels[selectedERP]}</p>
      <p class="text-xs text-gray-400 mb-4">Ingresa tus credenciales de API o conecta via OAuth. Remora solo solicita acceso de lectura.</p>
      {#if selectedERP === 'quickbooks' || selectedERP === 'xero'}
        <button class="flex items-center gap-2 border border-gray-200 rounded-lg px-5 py-3 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors"
          type="button" on:click={handleConnect} disabled={connecting || connected}>
          {#if connecting}Conectando…{:else if connected}✓ Conectado{:else}Conectar con OAuth{/if}
        </button>
      {:else}
        <div class="flex flex-col gap-3">
          <input id="api-key" type="password" placeholder="Pega tu API key aquí" bind:value={apiKey}
            class="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#FE42A0] placeholder:text-gray-300" />
          <button class="bg-[#0B122A] text-white px-5 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors disabled:opacity-40 w-fit"
            type="button" disabled={!apiKey || connecting || connected} on:click={handleConnect}>
            {#if connecting}Verificando…{:else if connected}✓ Verificado{:else}Verificar y conectar{/if}
          </button>
        </div>
      {/if}
    </div>
  {/if}

  <div class="flex justify-end pt-4 pb-8">
    <button class="bg-[#0B122A] text-white px-6 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors" type="button">Continuar</button>
  </div>
</div>
