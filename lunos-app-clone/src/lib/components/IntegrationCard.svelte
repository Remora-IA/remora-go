<script>
  export let logo = ''
  export let label = ''
  export let alt = ''
  export let selected = false
  export let bordered = false
  export let onClick = () => {}
</script>
<button
  class="flex items-center justify-center rounded-lg border-2 p-4 h-20 w-full transition-all cursor-pointer
    {selected ? 'border-[#FE42A0] bg-pink-50' : bordered ? 'border-[#FE42A0] bg-white hover:bg-pink-50' : 'border-gray-200 bg-white hover:border-gray-300 hover:shadow-sm'}"
  on:click={onClick} type="button">
  {#if logo}
    <img src={logo} alt={alt} class="max-h-8 max-w-[80%] object-contain" />
  {:else}
    <span class="text-sm font-medium text-gray-700">{label}</span>
  {/if}
</button>
