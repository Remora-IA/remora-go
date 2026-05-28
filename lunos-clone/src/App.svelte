<script>
  import Header from './lib/components/Header.svelte'
  import Hero from './lib/components/Hero.svelte'
  import Features from './lib/components/Features.svelte'
  import PainPoints from './lib/components/PainPoints.svelte'
  import Testimonials from './lib/components/Testimonials.svelte'
  import HowItWorks from './lib/components/HowItWorks.svelte'
  import Benefits from './lib/components/Benefits.svelte'
  import TeamRoles from './lib/components/TeamRoles.svelte'
  import CTA from './lib/components/CTA.svelte'
  import FAQ from './lib/components/FAQ.svelte'
  import Footer from './lib/components/Footer.svelte'
  import Pricing from './lib/components/Pricing.svelte'
  import About from './lib/components/About.svelte'

  // Simple client-side routing — no library needed
  let page = window.location.pathname

  // Intercept clicks on internal links to do SPA navigation
  function handleClick(e) {
    const a = e.target.closest('a')
    if (!a) return
    const href = a.getAttribute('href')
    if (!href || href.startsWith('http') || href.startsWith('#') || a.target === '_blank') return
    e.preventDefault()
    window.history.pushState({}, '', href)
    page = href
    window.scrollTo(0, 0)
  }

  window.addEventListener('popstate', () => { page = window.location.pathname })
</script>

<!-- svelte-ignore a11y-click-events-have-key-events -->
<!-- svelte-ignore a11y-no-static-element-interactions -->
<div on:click={handleClick}>
  <Header />
  {#if page === '/pricing'}
    <main class="main">
      <Pricing />
    </main>
  {:else if page === '/about'}
    <main class="main">
      <About />
    </main>
  {:else}
    <main class="main">
      <Hero />
      <Features />
      <PainPoints />
      <Testimonials />
      <HowItWorks />
      <Benefits />
      <TeamRoles />
      <CTA />
      <FAQ />
    </main>
  {/if}
  <Footer />
</div>
