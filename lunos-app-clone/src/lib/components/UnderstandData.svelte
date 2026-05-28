<script>
  import IntegrationCard from './IntegrationCard.svelte'
  let sourceSelections = new Set()
  let contactSelections = new Set()
  let paymentSelections = new Set()
  let chatSelections = new Set()

  function toggle(set, id) {
    const next = new Set(set)
    if (next.has(id)) next.delete(id); else next.add(id)
    return next
  }

  const sourceIntegrations = [
    { id: 'quickbooks', logo: '/logos/quickbooks.svg', alt: 'QuickBooks' },
    { id: 'xero',       logo: '/logos/xero.svg',       alt: 'Xero' },
    { id: 'netsuite',   logo: '/logos/netsuite.svg',   alt: 'NetSuite' },
    { id: 'stripe',     logo: '/logos/stripe.svg',     alt: 'Stripe' },
    { id: 'maxio',      logo: '/logos/maxio.svg',      alt: 'Maxio' },
    { id: 'light',      logo: '/logos/light.svg',      alt: 'Light.inc' },
    { id: 'sage',       logo: '/logos/sage.svg',       alt: 'Sage' },
    { id: 'zoho',       logo: '/logos/zoho.svg',       alt: 'Zoho' },
    { id: 'chargebee',  logo: '/logos/chargebee.svg',  alt: 'Chargebee' },
    { id: 'zuora',      logo: '/logos/zuora.svg',      alt: 'Zuora' },
    { id: 'other',      label: 'Otro' },
    { id: 'none',       label: 'Ninguno', bordered: true },
  ]
  const contactIntegrations = [
    { id: 'hubspot',    logo: '/logos/hubspot.svg',    alt: 'HubSpot' },
    { id: 'salesforce', logo: '/logos/salesforce.svg', alt: 'Salesforce' },
    { id: 'other',      label: 'Otro' },
    { id: 'none',       label: 'Ninguno', bordered: true },
  ]
  const paymentIntegrations = [
    { id: 'stripe',     logo: '/logos/stripe.svg',     alt: 'Stripe' },
    { id: 'quickbooks', logo: '/logos/quickbooks.svg', alt: 'QuickBooks' },
    { id: 'xero',       logo: '/logos/xero.svg',       alt: 'Xero' },
    { id: 'other',      label: 'Otro' },
    { id: 'bank',       label: 'Solo me pagan en mi banco' },
  ]
  const chatIntegrations = [
    { id: 'slack',  logo: '/logos/slack.svg',  alt: 'Slack' },
    { id: 'teams',  logo: '/logos/teams.svg',  alt: 'Microsoft Teams' },
    { id: 'other',  label: 'Otro' },
    { id: 'none',   label: 'Ninguno', bordered: true },
  ]
</script>

<div class="flex flex-col gap-4 max-w-4xl">
  <p class="text-sm text-gray-500">Conecta Remora a tus sistemas</p>
  <h1 class="text-2xl font-semibold text-gray-900 mb-4">Entendamos tu stack</h1>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <div class="flex items-center gap-2 mb-4">
      <p class="text-sm font-medium text-gray-800">¿A qué fuentes de datos debe conectarse Remora para tus agentes de IA?</p>
    </div>
    <div class="grid grid-cols-3 sm:grid-cols-4 md:grid-cols-6 gap-3">
      {#each sourceIntegrations as item}
        <IntegrationCard {...item} selected={sourceSelections.has(item.id)}
          onClick={() => sourceSelections = toggle(sourceSelections, item.id)} />
      {/each}
    </div>
  </div>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-4">¿Qué canales de comunicación debe integrar Remora con tu equipo?</p>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
      {#each contactIntegrations as item}
        <IntegrationCard {...item} selected={contactSelections.has(item.id)}
          onClick={() => contactSelections = toggle(contactSelections, item.id)} />
      {/each}
    </div>
  </div>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-4">¿Usas algún proveedor de pagos?</p>
    <div class="grid grid-cols-2 sm:grid-cols-5 gap-3">
      {#each paymentIntegrations as item}
        <IntegrationCard {...item} selected={paymentSelections.has(item.id)}
          onClick={() => paymentSelections = toggle(paymentSelections, item.id)} />
      {/each}
    </div>
  </div>

  <div class="border border-gray-200 rounded-xl p-6 bg-white">
    <p class="text-sm font-medium text-gray-800 mb-4">¿Qué plataforma de chat usa tu equipo para comunicación interna?</p>
    <div class="grid grid-cols-2 sm:grid-cols-4 gap-3">
      {#each chatIntegrations as item}
        <IntegrationCard {...item} selected={chatSelections.has(item.id)}
          onClick={() => chatSelections = toggle(chatSelections, item.id)} />
      {/each}
    </div>
  </div>

  <div class="flex justify-end pt-4 pb-8">
    <button class="bg-[#0B122A] text-white px-6 py-2.5 rounded-md text-sm font-medium hover:bg-[#1a2540] transition-colors" type="button">
      Continuar
    </button>
  </div>
</div>
