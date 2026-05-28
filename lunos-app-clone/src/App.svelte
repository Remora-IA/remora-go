<script>
  import AppNavbar from './lib/components/AppNavbar.svelte'
  import OnboardingSidebar from './lib/components/OnboardingSidebar.svelte'
  import OnboardingLanding from './lib/components/OnboardingLanding.svelte'
  import UnderstandData from './lib/components/UnderstandData.svelte'
  import ConnectERP from './lib/components/ConnectERP.svelte'
  import AddPayment from './lib/components/AddPayment.svelte'
  import AddBranding from './lib/components/AddBranding.svelte'

  let view = 'landing'
  let activeStep = 'understand'

  function handleCardSelect(id) {
    if (id === 'connect') { view = 'form'; activeStep = 'understand' }
  }
  function handleStepSelect(step) { activeStep = step; view = 'form' }
</script>

<AppNavbar userName="TB" />

{#if view === 'form'}
  <OnboardingSidebar {activeStep} onSelectStep={handleStepSelect} />
{/if}

<main class="flex-1 min-w-0 pt-[76px] overflow-auto {view === 'form' ? 'pl-64' : ''}">
  {#if view === 'landing'}
    <OnboardingLanding userName="Tomás" onSelectCard={handleCardSelect} />
  {:else}
    <div class="p-8">
      {#if activeStep === 'understand'}
        <UnderstandData />
      {:else if activeStep === 'erp'}
        <ConnectERP />
      {:else if activeStep === 'payment'}
        <AddPayment />
      {:else if activeStep === 'branding'}
        <AddBranding />
      {/if}
    </div>
  {/if}
</main>

<style>
  :global(body) { margin: 0; padding: 0; background: white; }
  :global(#app) { display: flex; flex-direction: column; min-height: 100vh; }
</style>
