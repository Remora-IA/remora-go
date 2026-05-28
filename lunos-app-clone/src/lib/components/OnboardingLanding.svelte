<script>
  export let userName = 'Tomás'
  export let onSelectCard = (id) => {}
  const cards = [
    { id: 'connect', number: '1', label: 'Conecta Remora a tus sistemas', progress: 0 },
    { id: 'action',  number: '2', label: 'Despliega tu primer agente', progress: 0 },
    { id: 'finish',  number: '3', label: 'Termina de configurar Remora', progress: 0 },
    { id: 'chat',    icon: true,  label: 'Chatea con Remora', purple: true },
  ]
</script>
<div class="min-h-screen flex flex-col w-full bg-white">
  <div class="flex flex-col p-8 gap-8 max-w-7xl mx-auto w-full">
    <div class="space-y-6">
      <div class="text-center py-8">
        <h1 class="text-3xl font-medium text-gray-900">Hola {userName}, completa las tareas de abajo</h1>
        <p class="text-3xl font-medium text-gray-900 mt-1">para configurar Remora</p>
      </div>
      <div class="w-full">
        <div class="grid grid-cols-1 md:grid-cols-2 lg:grid-cols-4 gap-6">
          {#each cards as card}
            <div class="p-6 rounded-lg transition-shadow relative h-64 flex flex-col cursor-pointer hover:shadow-md
                {card.purple ? 'bg-purple-100' : 'bg-gray-100'}"
              on:click={() => onSelectCard(card.id)} role="button" tabindex="0">
              {#if card.icon}
                <div class="mb-4">
                  <div class="w-12 h-12 rounded-full bg-purple-600 flex items-center justify-center">
                    <svg class="w-6 h-6 text-white" fill="none" viewBox="0 0 24 24" stroke="currentColor">
                      <path stroke-linecap="round" stroke-linejoin="round" stroke-width="2"
                        d="M8 12h.01M12 12h.01M16 12h.01M21 12c0 4.418-4.03 8-9 8a9.863 9.863 0 01-4.255-.949L3 20l1.395-3.72C3.512 15.042 3 13.574 3 12c0-4.418 4.03-8 9-8s9 3.582 9 8z"/>
                    </svg>
                  </div>
                </div>
                <div class="flex-1"></div>
                <h3 class="text-xl font-medium text-gray-900">{card.label}</h3>
              {:else}
                <div class="mb-8">
                  <div class="text-6xl font-light text-gray-900 leading-none">{card.number}</div>
                </div>
                <div class="flex-1"></div>
                <div class="mb-6">
                  <div class="h-2 w-full rounded-full bg-gray-200 overflow-hidden">
                    <div class="h-2 bg-[#0B122A] rounded-full transition-all" style="width:{card.progress}%"></div>
                  </div>
                </div>
                <h3 class="text-xl font-medium text-gray-900 leading-snug">{card.label}</h3>
              {/if}
            </div>
          {/each}
        </div>
      </div>
    </div>
  </div>
</div>
