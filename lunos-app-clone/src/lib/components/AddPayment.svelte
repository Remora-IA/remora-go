<script>
  let selectedMethod = null
  let bankName = ''; let accountNumber = ''; let routingNumber = ''
  let connecting = false; let connected = false

  const methods = [
    { id: 'stripe', title: 'Stripe',         description: 'Acepta tarjetas de crédito, débito, ACH y más',  color: '#635BFF' },
    { id: 'ach',    title: 'ACH / Transferencia bancaria', description: 'Permite a tus clientes pagar directo desde su banco', color: '#0B122A' },
    { id: 'check',  title: 'Cheque',          description: 'Registra pagos recibidos por cheque físico',   color: '#6B7280' },
    { id: 'wire',   title: 'Transferencia',  description: 'Recibe transferencias nacionales e internacionales',    color: '#0EA5E9' },
  ]

  async function handleConnect() {
    connecting = true
    await new Promise(r => setTimeout(r, 1000))
    connecting = false; connected = true
  }
</script>

<div class="flex flex-col gap-4 max-w-4xl">
  <p class="text-sm text-gray-500">Conecta Remora a tus sistemas</p>
  <h1 class="text-2xl font-semibold text-gray-900 mb-4">Configura integraciones</h1>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-1">¿Cómo te pagan tus clientes?</p>
    <p class="text-xs text-gray-400 mb-4">Selecciona todos los que apliquen.</p>
    <div class="grid grid-cols-2 gap-3">
      {#each methods as m}
        <button type="button" class="flex items-center gap-4 p-4 rounded-xl border-2 text-left transition-all
            {selectedMethod === m.id ? 'border-[#FE42A0] bg-pink-50' : 'border-gray-200 bg-white hover:border-gray-300 hover:shadow-sm'}"
          on:click={() => { selectedMethod = m.id; connected = false }}>
          <div style="color:{m.color}" class="flex-shrink-0">
            <svg class="w-6 h-6" fill="none" viewBox="0 0 24 24" stroke="currentColor" stroke-width="1.8">
              <rect x="2" y="5" width="20" height="14" rx="2"/><path stroke-linecap="round" d="M2 10h20"/>
            </svg>
          </div>
          <div>
            <p class="text-sm font-semibold text-gray-900">{m.title}</p>
            <p class="text-xs text-gray-400 mt-0.5">{m.description}</p>
          </div>
        </button>
      {/each}
    </div>
  </div>

  {#if selectedMethod}
    <div class="border border-gray-200 rounded-xl p-6 bg-white">
      {#if selectedMethod === 'stripe'}
        <p class="text-sm font-medium text-gray-800 mb-1">Conectar Stripe</p>
        <p class="text-xs text-gray-400 mb-4">Remora solicitará acceso de solo lectura a tu cuenta de Stripe.</p>
        <button class="border border-gray-200 rounded-lg px-5 py-3 text-sm font-medium text-gray-700 hover:bg-gray-50 transition-colors disabled:opacity-40"
          type="button" disabled={connecting || connected} on:click={handleConnect}>
          {connecting ? 'Conectando…' : connected ? '✓ Conectado' : 'Conectar con Stripe OAuth'}
        </button>
      {:else if selectedMethod === 'ach'}
        <p class="text-sm font-medium text-gray-800 mb-1">Datos de ACH / Transferencia bancaria</p>
        <p class="text-xs text-gray-400 mb-4">Esta información se usa para la configuración de tu integración.</p>
        <div class="flex flex-col gap-3 max-w-md">
          <input bind:value={bankName} type="text" placeholder="Nombre del banco"
            class="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#FE42A0] placeholder:text-gray-300" />
          <input bind:value={accountNumber} type="text" placeholder="Número de cuenta"
            class="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#FE42A0] placeholder:text-gray-300" />
          <input bind:value={routingNumber} type="text" placeholder="Número de ruta"
            class="w-full border border-gray-200 rounded-lg px-4 py-2.5 text-sm focus:outline-none focus:ring-2 focus:ring-[#FE42A0] placeholder:text-gray-300" />
          <button class="bg-[#0B122A] text-white px-5 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors disabled:opacity-40 w-fit"
            type="button" disabled={!bankName || !accountNumber || !routingNumber} on:click={handleConnect}>Guardar datos bancarios</button>
        </div>
      {:else}
        <p class="text-sm font-medium text-gray-800 mb-4">{selectedMethod === 'check' ? 'Pagos con cheque' : 'Transferencias'}</p>
        <button class="bg-[#0B122A] text-white px-5 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors"
          type="button" on:click={handleConnect}>{connected ? '✓ Habilitado' : 'Habilitar este método'}</button>
      {/if}
    </div>
  {/if}

  <div class="flex justify-end pt-4 pb-8">
    <button class="bg-[#0B122A] text-white px-6 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors" type="button">Continuar</button>
  </div>
</div>
