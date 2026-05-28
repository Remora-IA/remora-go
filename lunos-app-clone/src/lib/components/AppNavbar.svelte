<script>
  export let userName = 'TB'
</script>
<header class="h-[76px] w-full fixed top-0 left-0 right-0 z-30 flex items-center justify-between bg-[#030712] px-6">
  <a href="/" class="flex items-center gap-2">
    <svg width="28" height="28" viewBox="0 0 28 28" fill="none">
      <path d="M14 2C14 8.627 8.627 14 2 14" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M8.5 2C8.5 5.59 5.59 8.5 2 8.5" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M14 2C14 8.627 19.373 14 26 14" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M19.5 2C19.5 5.59 22.41 8.5 26 8.5" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M14 26C14 19.373 8.627 14 2 14" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M8.5 26C8.5 22.41 5.59 19.5 2 19.5" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M14 26C14 19.373 19.373 14 26 14" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
      <path d="M19.5 26C19.5 22.41 22.41 19.5 26 19.5" stroke="#FE42A0" stroke-width="2.5" stroke-linecap="square"/>
    </svg>
    <span class="text-white text-xl font-semibold tracking-wide">REMORA</span>
  </a>
  <button class="w-9 h-9 rounded-full bg-violet-500 flex items-center justify-center text-white text-sm font-semibold">
    {userName}
  </button>
</header>
