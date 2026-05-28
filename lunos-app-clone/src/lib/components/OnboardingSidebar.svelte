<script>
  export let activeStep = 'understand'
  export let onSelectStep = (step) => {}
  const steps = [
    { id: 'connect', label: 'Conecta Remora a tus sistemas', expanded: true,
      substeps: [
        { id: 'understand', label: 'Entendamos tu stack' },
        { id: 'erp',        label: 'Conecta tus fuentes de datos' },
        { id: 'payment',    label: 'Configura integraciones' },
        { id: 'branding',   label: 'Personaliza tu espacio' },
      ]},
    { id: 'first-action', label: 'Despliega tu primer agente' },
    { id: 'finish',       label: 'Termina de configurar Remora' },
  ]
</script>
<aside class="w-64 min-w-[256px] h-screen fixed left-0 top-[76px] bg-white border-r border-gray-200 overflow-y-auto pb-8 z-20">
  <nav class="flex flex-col py-4">
    {#each steps as step}
      {#if step.substeps}
        <div class="px-4 py-3">
          <div class="flex items-center justify-between">
            <span class="text-sm font-medium text-gray-900">{step.label}</span>
            <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
              <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M19 9l-7 7-7-7"/>
            </svg>
          </div>
          {#if step.expanded}
            <div class="mt-2 flex flex-col gap-1">
              {#each step.substeps as sub}
                <button class="flex items-center gap-2 px-3 py-2 rounded-md text-sm w-full text-left transition-colors
                    {activeStep === sub.id ? 'bg-orange-100 text-orange-800 font-medium' : 'text-gray-600 hover:bg-gray-50'}"
                  on:click={() => onSelectStep(sub.id)}>
                  <span class="w-4 h-4 rounded-full border-2 flex-shrink-0 flex items-center justify-center
                    {activeStep === sub.id ? 'border-orange-500' : 'border-gray-300'}">
                    {#if activeStep === sub.id}
                      <span class="w-1.5 h-1.5 rounded-full bg-orange-500"></span>
                    {/if}
                  </span>
                  {sub.label}
                </button>
              {/each}
            </div>
          {/if}
        </div>
      {:else}
        <button class="flex items-center justify-between px-4 py-3 text-sm text-gray-500 hover:bg-gray-50 w-full text-left">
          <span>{step.label}</span>
          <svg class="w-4 h-4 text-gray-400" fill="none" viewBox="0 0 24 24" stroke="currentColor">
            <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2" d="M9 5l7 7-7 7"/>
          </svg>
        </button>
      {/if}
    {/each}
  </nav>
</aside>
