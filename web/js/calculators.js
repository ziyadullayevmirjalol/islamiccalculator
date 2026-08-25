// Declarative registry of all 16 calculators: form fields, payload
// mapping, and result rendering. The generic form engine in pages.js
// consumes these definitions.
import { el, fmtMoney, fmtPercent, kvRows, dataTable, badge, notice, sectionTitle } from './ui.js';
import { t } from './i18n.js';

const int = v => parseInt(v, 10);
const put = (obj, key, v) => { if (v !== '' && v !== undefined && v !== null) obj[key] = v; };

// Shared renderers ------------------------------------------------------------

function scheduleTable(schedule, withSplit) {
  const columns = [
    { key: 'n', label: '#' },
    { key: 'dueDate', label: 'Due date' },
    { key: 'amount', label: 'Amount', align: 'right' },
    ...(withSplit ? [
      { key: 'principal', label: 'Principal', align: 'right' },
      { key: 'markup', label: 'Markup', align: 'right' },
    ] : []),
    { key: 'balance', label: 'Balance', align: 'right' },
  ];
  return dataTable(columns, schedule.map(r => ({
    ...r,
    amount: fmtMoney(r.amount),
    principal: r.principal && fmtMoney(r.principal),
    markup: r.markup && fmtMoney(r.markup),
    balance: fmtMoney(r.balance),
  })));
}

function nisabBlock(d) {
  return [
    sectionTitle('Nisab'),
    kvRows([
      ['Gold-based nisab', fmtMoney(d.nisab.goldValue)],
      ['Silver-based nisab', fmtMoney(d.nisab.silverValue)],
      ['Applied threshold', fmtMoney(d.nisab.applied), { emphasis: true }],
      ['Basis', d.nisab.basis],
    ]),
  ];
}

function pricesBlock(prices) {
  const row = (label, p) => el('div', { class: 'flex items-center justify-between gap-3 py-1.5 text-sm' },
    el('span', { class: 'text-stone-500' }, label),
    el('span', { class: 'flex items-center gap-2' },
      el('span', { class: 'font-mono tabular-nums' }, fmtMoney(p.pricePerGram) + ' / g'),
      badge(p.source, 'stone'),
      p.stale && badge('STALE', 'red'),
    ));
  const anyStale = prices.gold.stale || prices.silver.stale;
  return [
    sectionTitle('Market prices used'),
    row('Gold', prices.gold),
    row('Silver', prices.silver),
    anyStale && notice('Price data is outdated — the nisab and zakat figures may not reflect current markets.'),
  ];
}

const notGuaranteed = () =>
  notice('Expected profit — not guaranteed. An Islamic deposit/sukuk never promises a return.', 'info');

// The registry ----------------------------------------------------------------

export const GROUPS = ['Retail finance', 'Business finance', 'Zakat & religious', 'Investment'];

export const CALCS = [

  // --- Retail finance --------------------------------------------------------
  {
    id: 'murabaha', group: 'Retail finance',
    title: 'Murabaha', blurb: 'Cost-plus installment sale — car, appliances, housing.',
    endpoint: '/api/v1/finance/murabaha',
    fields: [
      { name: 'cost', label: 'Asset cost', type: 'money', placeholder: '120000000' },
      { name: 'markupMode', label: 'Markup as', type: 'select', default: 'rate',
        options: [{ value: 'rate', label: '% of cost' }, { value: 'amount', label: 'Fixed amount' }] },
      { name: 'markupValue', label: 'Markup value', type: 'money', placeholder: '0.10',
        hint: v => v.markupMode === 'rate' ? 'e.g. 0.10 = 10%' : 'absolute amount' },
      { name: 'downPayment', label: 'Down payment (optional)', type: 'money' },
      { name: 'termMonths', label: 'Term (months)', type: 'int', placeholder: '12' },
      { name: 'firstDueDate', label: 'First due date (optional)', type: 'date' },
    ],
    payload(v) {
      const p = { cost: v.cost, markup: { mode: v.markupMode, value: v.markupValue }, termMonths: int(v.termMonths) };
      put(p, 'downPayment', v.downPayment);
      put(p, 'firstDueDate', v.firstDueDate);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Sale price (fixed at contract)', fmtMoney(d.salePrice), { emphasis: true }],
          ['Total markup', fmtMoney(d.markupTotal), { tone: 'brass' }],
          ['Down payment', fmtMoney(d.downPayment)],
          ['Financed', fmtMoney(d.financed)],
          ['Monthly installment', fmtMoney(d.monthlyInstallment), { emphasis: true }],
        ]),
        sectionTitle(`${t('Schedule')} — ${d.termMonths} ${t('installments')}`),
        scheduleTable(d.schedule, true),
      ];
    },
  },

  {
    id: 'ijara', group: 'Retail finance',
    title: 'Ijara Muntahia Bittamleek', blurb: 'Lease-to-own: rent now, ownership transfer at term end.',
    endpoint: '/api/v1/finance/ijara',
    fields: [
      { name: 'mode', label: 'Calculate from', type: 'select', default: 'profit',
        options: [{ value: 'profit', label: 'Target profit → derive rent' }, { value: 'rent', label: 'Known rent → derive profit' }] },
      { name: 'assetCost', label: 'Asset cost', type: 'money', placeholder: '100000000' },
      { name: 'profitMode', label: 'Profit as', type: 'select', default: 'rate', showIf: v => v.mode === 'profit',
        options: [{ value: 'rate', label: '% of cost' }, { value: 'amount', label: 'Fixed amount' }] },
      { name: 'profitValue', label: 'Profit value', type: 'money', placeholder: '0.20', showIf: v => v.mode === 'profit' },
      { name: 'monthlyRent', label: 'Monthly rent', type: 'money', showIf: v => v.mode === 'rent' },
      { name: 'transferPrice', label: 'Ownership transfer price', type: 'money' },
      { name: 'advancePayment', label: 'Advance rent (optional)', type: 'money' },
      { name: 'termMonths', label: 'Term (months)', type: 'int', placeholder: '24' },
      { name: 'firstDueDate', label: 'First due date (optional)', type: 'date' },
    ],
    payload(v) {
      const p = { mode: v.mode, assetCost: v.assetCost, termMonths: int(v.termMonths) };
      if (v.mode === 'profit') p.profit = { mode: v.profitMode, value: v.profitValue };
      else put(p, 'monthlyRent', v.monthlyRent);
      put(p, 'transferPrice', v.transferPrice);
      put(p, 'advancePayment', v.advancePayment);
      put(p, 'firstDueDate', v.firstDueDate);
      return p;
    },
    render(d) {
      const loss = d.profitTotal.startsWith('-');
      return [
        kvRows([
          ['Monthly rent', fmtMoney(d.monthlyRent), { emphasis: true }],
          ['Total rentals', fmtMoney(d.totalRentals)],
          ['Transfer price at term end', fmtMoney(d.transferPrice)],
          ['Advance payment', fmtMoney(d.advancePayment)],
          ['Total cost of ownership', fmtMoney(d.totalReceipts), { emphasis: true }],
          [loss ? 'Loss' : 'Lessor profit', fmtMoney(d.profitTotal), { tone: loss ? 'neg' : 'brass' }],
          ['Profit rate', fmtPercent(d.profitRate)],
        ]),
        loss && notice('Rent below cost recovery — this lease loses money for the lessor.'),
        sectionTitle('Rent schedule'),
        scheduleTable(d.schedule, false),
      ];
    },
  },

  {
    id: 'qard-hasan', group: 'Retail finance',
    title: 'Qard al-Hasan', blurb: 'Benevolent loan — repay exactly the principal, fixed service fee only.',
    endpoint: '/api/v1/finance/qard-hasan',
    fields: [
      { name: 'principal', label: 'Principal', type: 'money', placeholder: '10000000' },
      { name: 'serviceFee', label: 'Fixed service fee (optional)', type: 'money',
        hint: () => 'A fixed amount only — a percentage would be riba, the API rejects the concept.' },
      { name: 'termMonths', label: 'Term (months)', type: 'int', placeholder: '10' },
      { name: 'firstDueDate', label: 'First due date (optional)', type: 'date' },
    ],
    payload(v) {
      const p = { principal: v.principal, termMonths: int(v.termMonths) };
      put(p, 'serviceFee', v.serviceFee);
      put(p, 'firstDueDate', v.firstDueDate);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Principal', fmtMoney(d.principal)],
          ['Service fee (upfront)', fmtMoney(d.serviceFee)],
          ['Total repayment', fmtMoney(d.totalRepayment), { emphasis: true }],
          ['Monthly installment', fmtMoney(d.monthlyInstallment), { emphasis: true }],
        ]),
        sectionTitle('Repayment schedule (principal only)'),
        scheduleTable(d.schedule, false),
      ];
    },
  },

  {
    id: 'mudaraba', group: 'Retail finance',
    title: 'Mudaraba / Wakala deposit', blurb: 'Expected profit on a profit-sharing or agency deposit.',
    endpoint: '/api/v1/finance/mudaraba',
    fields: [
      { name: 'mode', label: 'Deposit type', type: 'select', default: 'mudaraba',
        options: [{ value: 'mudaraba', label: 'Mudaraba (profit sharing)' }, { value: 'wakala', label: 'Wakala (agency)' }] },
      { name: 'amount', label: 'Deposit amount', type: 'money', placeholder: '10000000' },
      { name: 'poolRateAnnual', label: 'Indicative pool rate (annual)', type: 'money', placeholder: '0.18' },
      { name: 'shareRatio', label: 'Your profit share (0–1)', type: 'money', placeholder: '0.60', showIf: v => v.mode === 'mudaraba' },
      { name: 'wakalaFeeRate', label: 'Wakala fee rate (annual)', type: 'money', placeholder: '0.02', showIf: v => v.mode === 'wakala' },
      { name: 'termMonths', label: 'Term (months)', type: 'int', placeholder: '12' },
    ],
    payload(v) {
      const p = { mode: v.mode, amount: v.amount, poolRateAnnual: v.poolRateAnnual, termMonths: int(v.termMonths) };
      if (v.mode === 'mudaraba') put(p, 'shareRatio', v.shareRatio);
      else put(p, 'wakalaFeeRate', v.wakalaFeeRate);
      return p;
    },
    render(d) {
      return [
        notGuaranteed(),
        kvRows([
          ['Effective annual rate', fmtPercent(d.effectiveAnnualRate)],
          ['Expected profit', fmtMoney(d.expectedProfit), { emphasis: true }],
          ['≈ per month', fmtMoney(d.expectedMonthlyProfit)],
          ['Expected total at maturity', fmtMoney(d.expectedTotal), { emphasis: true }],
        ]),
      ];
    },
  },

  {
    id: 'diminishing-musharaka', group: 'Retail finance',
    title: 'Diminishing Musharakah', blurb: 'Islamic home financing: rent the bank’s share while buying it out.',
    endpoint: '/api/v1/finance/diminishing-musharaka',
    fields: [
      { name: 'propertyValue', label: 'Property value', type: 'money', placeholder: '300000000' },
      { name: 'downPayment', label: 'Down payment (your initial share)', type: 'money' },
      { name: 'annualRentalRate', label: 'Annual rental rate (on bank’s share)', type: 'money', placeholder: '0.05' },
      { name: 'termMonths', label: 'Term (months)', type: 'int', placeholder: '240' },
    ],
    payload(v) {
      const p = { propertyValue: v.propertyValue, annualRentalRate: v.annualRentalRate, termMonths: int(v.termMonths) };
      put(p, 'downPayment', v.downPayment);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Bank financing', fmtMoney(d.bankFinancing)],
          ['Your starting ownership', fmtPercent(d.initialOwnershipPercent)],
          ['Monthly share purchase', fmtMoney(d.monthlyAcquisition)],
          ['First month payment', fmtMoney(d.firstMonthPayment), { emphasis: true }],
          ['Total rent over the term', fmtMoney(d.totalRent), { tone: 'brass' }],
          ['Total paid (incl. down payment)', fmtMoney(d.totalPaid), { emphasis: true }],
        ]),
        notice('Rent is charged only on the bank’s outstanding share — it declines every month until you own 100%.', 'info'),
        sectionTitle('Monthly schedule'),
        dataTable(
          [{ key: 'n', label: '#' }, { key: 'bankShareBefore', label: 'Bank share', align: 'right' },
           { key: 'rent', label: 'Rent', align: 'right' }, { key: 'acquisition', label: 'Acquisition', align: 'right' },
           { key: 'payment', label: 'Payment', align: 'right' }, { key: 'ownershipPercent', label: 'Ownership', align: 'right' }],
          d.schedule.map(m => ({
            n: m.n, bankShareBefore: fmtMoney(m.bankShareBefore), rent: fmtMoney(m.rent),
            acquisition: fmtMoney(m.acquisition), payment: fmtMoney(m.payment),
            ownershipPercent: fmtPercent(m.ownershipPercent),
          })),
        ),
      ];
    },
  },

  // --- Business finance ------------------------------------------------------
  {
    id: 'salam', group: 'Business finance',
    title: 'Salam', blurb: 'Advance-payment purchase of a future harvest.',
    endpoint: '/api/v1/finance/salam',
    fields: [
      { name: 'quantity', label: 'Quantity (units)', type: 'money', placeholder: '100' },
      { name: 'unitPrice', label: 'Contracted advance price / unit', type: 'money', placeholder: '2400000' },
      { name: 'expectedUnitPrice', label: 'Expected market price at delivery / unit', type: 'money', placeholder: '3000000' },
      { name: 'deliveryDate', label: 'Delivery date (optional)', type: 'date' },
    ],
    payload(v) {
      const p = { quantity: v.quantity, unitPrice: v.unitPrice, expectedUnitPrice: v.expectedUnitPrice };
      put(p, 'deliveryDate', v.deliveryDate);
      return p;
    },
    render(d) {
      const neg = d.expectedMargin.startsWith('-');
      return [
        notice('Salam requires the FULL price paid at contract — partial advance is not valid Salam.', 'info'),
        kvRows([
          ['Advance paid now (full)', fmtMoney(d.advanceTotal), { emphasis: true }],
          ['Expected market value at delivery', fmtMoney(d.expectedMarketValue)],
          ['Expected margin', fmtMoney(d.expectedMargin), { tone: neg ? 'neg' : 'brass' }],
          ['Margin rate', fmtPercent(d.marginRate)],
          d.deliveryDate ? ['Delivery date', d.deliveryDate] : null,
        ]),
        neg && notice('Expected price is below the advance — the buyer bears this risk.'),
      ];
    },
  },

  {
    id: 'istisna', group: 'Business finance',
    title: 'Istisna', blurb: 'Construction / manufacturing with milestone-staged payments.',
    endpoint: '/api/v1/finance/istisna',
    fields: [
      { name: 'mode', label: 'Payment plan', type: 'select', default: 'milestones',
        options: [{ value: 'milestones', label: 'Named milestones' }, { value: 'equal', label: 'Equal stages' }] },
      { name: 'contractPrice', label: 'Contract price', type: 'money', placeholder: '90000000' },
      { name: 'stages', label: 'Number of equal stages', type: 'int', placeholder: '3', showIf: v => v.mode === 'equal' },
    ],
    list: {
      name: 'milestones', label: 'Milestones (percents must sum to 100)',
      showIf: v => v.mode === 'milestones',
      columns: [
        { name: 'name', label: 'Name', type: 'text', placeholder: 'foundation' },
        { name: 'percent', label: '%', type: 'money', placeholder: '30' },
        { name: 'dueDate', label: 'Due date', type: 'date' },
      ],
      defaultRows: [{ name: 'foundation', percent: '30' }, { name: 'structure', percent: '50' }, { name: 'handover', percent: '20' }],
    },
    payload(v) {
      const p = { mode: v.mode, contractPrice: v.contractPrice };
      if (v.mode === 'equal') p.stages = int(v.stages);
      else p.milestones = (v.milestones || []).map(m => {
        const row = { percent: m.percent };
        put(row, 'name', m.name);
        put(row, 'dueDate', m.dueDate);
        return row;
      });
      return p;
    },
    render(d) {
      return [
        kvRows([['Contract price', fmtMoney(d.contractPrice), { emphasis: true }], ['Stages', String(d.stages)]]),
        sectionTitle('Stage payments'),
        dataTable(
          [{ key: 'n', label: '#' }, { key: 'name', label: 'Milestone' }, { key: 'percent', label: '%', align: 'right' },
           { key: 'dueDate', label: 'Due date' }, { key: 'amount', label: 'Amount', align: 'right' }],
          d.schedule.map(s => ({ ...s, amount: fmtMoney(s.amount) })),
        ),
      ];
    },
  },

  {
    id: 'musharaka', group: 'Business finance',
    title: 'Musharaka', blurb: 'Partnership: profit by agreement, loss strictly by capital.',
    endpoint: '/api/v1/finance/musharaka',
    fields: [
      { name: 'resultType', label: 'Period result', type: 'select', default: 'profit',
        options: [{ value: 'profit', label: 'Profit' }, { value: 'loss', label: 'Loss' }] },
      { name: 'amount', label: 'Amount to distribute', type: 'money', placeholder: '20000000' },
    ],
    list: {
      name: 'partners', label: 'Partners (profit shares must sum to 100)',
      columns: [
        { name: 'name', label: 'Name', type: 'text', placeholder: 'A' },
        { name: 'capital', label: 'Capital', type: 'money', placeholder: '70000000' },
        { name: 'profitSharePercent', label: 'Profit %', type: 'money', placeholder: '50' },
      ],
      defaultRows: [
        { name: 'A', capital: '70000000', profitSharePercent: '50' },
        { name: 'B', capital: '30000000', profitSharePercent: '50' },
      ],
    },
    payload(v) {
      return {
        resultType: v.resultType,
        amount: v.amount,
        partners: (v.partners || []).map(p => {
          const row = { capital: p.capital, profitSharePercent: p.profitSharePercent };
          put(row, 'name', p.name);
          return row;
        }),
      };
    },
    render(d) {
      return [
        el('div', { class: 'flex items-center gap-2 mb-2' },
          badge(d.resultType === 'loss' ? 'Loss' : 'Profit', d.resultType === 'loss' ? 'red' : 'green'),
          badge(d.basis === 'capital_ratio' ? 'split by capital' : 'split by agreed ratio', 'stone')),
        d.resultType === 'loss' && notice('Loss always follows capital shares — the agreed profit ratio does not apply to losses.', 'info'),
        kvRows([['Total capital', fmtMoney(d.totalCapital)], ['Distributed', fmtMoney(d.amount), { emphasis: true }]]),
        sectionTitle('Per-partner shares'),
        dataTable(
          [{ key: 'name', label: 'Partner' }, { key: 'capital', label: 'Capital', align: 'right' },
           { key: 'capitalShare', label: 'Capital %', align: 'right' }, { key: 'profitSharePercent', label: 'Agreed %', align: 'right' },
           { key: 'appliedShare', label: 'Applied', align: 'right' }, { key: 'amount', label: 'Share', align: 'right' }],
          d.shares.map(s => ({
            ...s, capital: fmtMoney(s.capital), capitalShare: fmtPercent(s.capitalShare),
            profitSharePercent: s.profitSharePercent + '%', appliedShare: fmtPercent(s.appliedShare),
            amount: fmtMoney(s.amount),
          })),
        ),
      ];
    },
  },

  {
    id: 'business-zakat', group: 'Business finance',
    title: 'Business zakat', blurb: '2.5% on net zakatable working capital.',
    endpoint: '/api/v1/zakat/business',
    fields: [
      { name: 'cash', label: 'Cash & bank', type: 'money' },
      { name: 'receivables', label: 'Receivables', type: 'money' },
      { name: 'inventory', label: 'Inventory at market value', type: 'money' },
      { name: 'shortTermLiabilities', label: 'Short-term liabilities', type: 'money' },
      { name: 'hawlComplete', label: 'A full lunar year (hawl) has passed', type: 'check' },
    ],
    payload(v) {
      const p = { hawlComplete: !!v.hawlComplete };
      put(p, 'cash', v.cash); put(p, 'receivables', v.receivables);
      put(p, 'inventory', v.inventory); put(p, 'shortTermLiabilities', v.shortTermLiabilities);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Zakat base (net working capital)', fmtMoney(d.zakatBase), { emphasis: true }],
          ['Above nisab', d.aboveNisab ? t('yes') : t('no')],
          ['Hawl complete', d.hawlComplete ? t('yes') : t('no')],
          [`Zakat due (${d.currency})`, fmtMoney(d.zakatDue), { emphasis: true }],
        ]),
        ...nisabBlock(d),
      ];
    },
  },

  {
    id: 'late-payment', group: 'Business finance',
    title: 'Late-payment charity', blurb: 'Deterrent on overdue installments — routed 100% to charity.',
    endpoint: '/api/v1/finance/late-payment',
    fields: [
      { name: 'mode', label: 'Charge type', type: 'select', default: 'rate',
        options: [{ value: 'rate', label: 'Annual rate, accrued daily' }, { value: 'flat', label: 'Flat fee' }] },
      { name: 'overdueAmount', label: 'Overdue amount', type: 'money', placeholder: '10000000' },
      { name: 'daysLate', label: 'Days late', type: 'int', placeholder: '30' },
      { name: 'annualRate', label: 'Annual rate', type: 'money', placeholder: '0.10', showIf: v => v.mode === 'rate' },
      { name: 'flatFee', label: 'Flat fee', type: 'money', showIf: v => v.mode === 'flat' },
    ],
    payload(v) {
      const p = { mode: v.mode, overdueAmount: v.overdueAmount, daysLate: int(v.daysLate) };
      if (v.mode === 'rate') put(p, 'annualRate', v.annualRate);
      else put(p, 'flatFee', v.flatFee);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Overdue amount', fmtMoney(d.overdueAmount)],
          ['Days late', String(d.daysLate)],
          ['Amount due → charity', fmtMoney(d.charityDue), { emphasis: true }],
        ]),
        notice('This amount is charity, never the financier’s income — an Islamic financier may not profit from delay.', 'info'),
      ];
    },
  },

  // --- Zakat & religious -----------------------------------------------------
  {
    id: 'wealth-zakat', group: 'Zakat & religious',
    title: 'Gold, silver & cash zakat', blurb: 'Nisab from live metal prices; 2.5% when the threshold is reached.',
    endpoint: '/api/v1/zakat/wealth',
    fields: [
      { name: 'goldGrams', label: 'Gold (grams)', type: 'money' },
      { name: 'silverGrams', label: 'Silver (grams)', type: 'money' },
      { name: 'cash', label: 'Cash & bank', type: 'money' },
      { name: 'otherAssets', label: 'Other zakatable assets', type: 'money' },
      { name: 'hawlComplete', label: 'A full lunar year (hawl) has passed', type: 'check' },
    ],
    payload(v) {
      const p = { hawlComplete: !!v.hawlComplete };
      put(p, 'goldGrams', v.goldGrams); put(p, 'silverGrams', v.silverGrams);
      put(p, 'cash', v.cash); put(p, 'otherAssets', v.otherAssets);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Gold value', fmtMoney(d.goldValue)],
          ['Silver value', fmtMoney(d.silverValue)],
          ['Total zakatable wealth', fmtMoney(d.totalWealth), { emphasis: true }],
          ['Above nisab', d.aboveNisab ? t('yes') : t('no')],
          ['Hawl complete', d.hawlComplete ? t('yes') : t('no')],
          [`Zakat due (${d.currency})`, fmtMoney(d.zakatDue), { emphasis: true }],
        ]),
        ...nisabBlock(d),
        ...pricesBlock(d.prices),
      ];
    },
  },

  {
    id: 'ushr', group: 'Zakat & religious',
    title: 'Ushr (harvest zakat)', blurb: '10% naturally watered, 5% irrigated.',
    endpoint: '/api/v1/zakat/ushr',
    fields: [
      { name: 'irrigationType', label: 'Water source', type: 'select', default: 'natural',
        options: [{ value: 'natural', label: 'Rain / natural water (10%)' }, { value: 'irrigated', label: 'Artificial irrigation (5%)' }] },
      { name: 'harvestValue', label: 'Harvest market value', type: 'money', placeholder: '20000000' },
    ],
    payload: v => ({ irrigationType: v.irrigationType, harvestValue: v.harvestValue }),
    render(d) {
      return [kvRows([
        ['Harvest value', fmtMoney(d.harvestValue)],
        ['Rate applied', fmtPercent(d.rate), { tone: 'brass' }],
        ['Ushr due', fmtMoney(d.ushrDue), { emphasis: true }],
      ])];
    },
  },

  {
    id: 'livestock', group: 'Zakat & religious',
    title: 'Livestock & silk-cocoon zakat', blurb: 'Sheep, cattle, camels by head count; cocoons by value.',
    endpoint: '/api/v1/zakat/livestock',
    fields: [
      { name: 'species', label: 'Type', type: 'select', default: 'sheep_goats',
        options: [
          { value: 'sheep_goats', label: 'Sheep & goats' }, { value: 'cattle', label: 'Cattle' },
          { value: 'camels', label: 'Camels' }, { value: 'silk_cocoons', label: 'Silk cocoons (pilla)' }] },
      { name: 'count', label: 'Head count', type: 'int', placeholder: '50', showIf: v => v.species !== 'silk_cocoons' },
      { name: 'marketValue', label: 'Cocoon market value', type: 'money', showIf: v => v.species === 'silk_cocoons' },
    ],
    payload(v) {
      const p = { species: v.species };
      if (v.species === 'silk_cocoons') put(p, 'marketValue', v.marketValue);
      else p.count = int(v.count);
      return p;
    },
    render(d) {
      const animalName = a => ({ sheep: 'sheep', tabi: "tabi' (1-year-old cow)", musinna: 'musinna (2-year-old cow)',
        bint_makhad: 'bint makhad (1-year she-camel)', bint_labun: 'bint labun (2-year she-camel)',
        hiqqa: 'hiqqa (3-year she-camel)', jadhaa: "jadha'a (4-year she-camel)" }[a] || a);
      if (d.belowNisab) {
        return [notice('Below nisab — no zakat is due on this herd.', 'info')];
      }
      if (d.cashDue) {
        return [kvRows([
          ['Rate', fmtPercent(d.rate), { tone: 'brass' }],
          ['Zakat due', fmtMoney(d.cashDue), { emphasis: true }],
        ]), notice('Cocoons are zakated as saleable produce.', 'info')];
      }
      return [
        sectionTitle('Animals due'),
        el('ul', { class: 'space-y-2' },
          d.due.map(x => el('li', { class: 'flex items-center gap-3 rounded-lg bg-brand-50 px-3 py-2' },
            el('span', { class: 'font-mono text-lg font-semibold text-brand-700' }, String(x.count)),
            el('span', {}, t(animalName(x.animal)))))),
        d.note === 'computed_by_combination_rule' &&
          notice("Computed by the per-30 / per-40 combination rule (tabi' per 30 head, musinna per 40).", 'info'),
        d.note === 'above_120_rules_vary_consult_scholar' &&
          notice('Above 120 camels the schools differ — please consult a scholar for the final ruling.'),
      ];
    },
  },

  {
    id: 'fidya', group: 'Zakat & religious',
    title: 'Fidya & Kaffarah', blurb: 'Compensation for missed fasts and broken oaths.',
    endpoint: '/api/v1/zakat/fidya',
    fields: [
      { name: 'kind', label: 'Type', type: 'select', default: 'fidya',
        options: [
          { value: 'fidya', label: 'Fidya — unable to fast (1 feeding/day)' },
          { value: 'kaffarah_fast', label: 'Kaffarah — broken fast (60 feedings)' },
          { value: 'kaffarah_oath', label: 'Kaffarah — broken oath (10 feedings)' }] },
      { name: 'count', label: 'Count (days / fasts / oaths)', type: 'int', placeholder: '1' },
    ],
    payload: v => ({ kind: v.kind, count: int(v.count) }),
    render(d) {
      return [
        kvRows([
          ['Feedings per unit', String(d.feedingsPerUnit)],
          [`Daily feeding rate (${d.currency})`, fmtMoney(d.dailyRate)],
          ['Total due', fmtMoney(d.totalDue), { emphasis: true }],
        ]),
        d.needsReview && notice('The feeding rate is a seeded value pending scholar approval.'),
      ];
    },
  },

  {
    id: 'fitrah', group: 'Zakat & religious',
    title: 'Zakat al-Fitr', blurb: 'Eid obligation per person — one sa’ of staple food or its value.',
    endpoint: '/api/v1/zakat/fitrah',
    fields: [
      { name: 'people', label: 'People covered (incl. infants)', type: 'int', placeholder: '5' },
      { name: 'peoplePaidInFood', label: 'Of them, paid in food', type: 'int', placeholder: '0' },
      { name: 'pricePerKg', label: 'Staple food price per kg', type: 'money', placeholder: '12000' },
      { name: 'saKg', label: 'One sa’ in kg (optional)', type: 'money',
        hint: () => 'Default 2.5 kg; scholarly estimates run 2.0–3.0 kg.' },
    ],
    payload(v) {
      const p = { people: int(v.people), pricePerKg: v.pricePerKg };
      if (v.peoplePaidInFood !== undefined && v.peoplePaidInFood !== '') p.peoplePaidInFood = int(v.peoplePaidInFood);
      put(p, 'saKg', v.saKg);
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Per person', fmtMoney(d.perPerson)],
          ['Total due', fmtMoney(d.totalDue), { emphasis: true }],
          ['Food to hand over (kg)', d.foodKg],
          ['Cash to hand over', fmtMoney(d.cashDue)],
        ]),
        notice('No nisab applies — Zakat al-Fitr is due for every covered person, and must reach recipients before the Eid prayer.', 'info'),
      ];
    },
  },

  {
    id: 'tazkiya', group: 'Zakat & religious',
    title: 'Tazkiya (income purification)', blurb: 'Identify impure/interest income and purge it to charity.',
    endpoint: '/api/v1/zakat/tazkiya',
    fields: [
      { name: 'mode', label: 'Mode', type: 'select', default: 'declared',
        options: [
          { value: 'declared', label: 'I know the impure amount' },
          { value: 'dividend', label: 'Purify a dividend by company ratio' }] },
      { name: 'totalIncome', label: 'Total income', type: 'money', showIf: v => v.mode === 'declared' },
      { name: 'impureAmount', label: 'Impure / interest portion', type: 'money', showIf: v => v.mode === 'declared' },
      { name: 'dividendAmount', label: 'Dividend received', type: 'money', showIf: v => v.mode === 'dividend' },
      { name: 'impureRatio', label: 'Company impure-income ratio (0–1)', type: 'money', placeholder: '0.03', showIf: v => v.mode === 'dividend' },
    ],
    payload(v) {
      const p = { mode: v.mode };
      if (v.mode === 'declared') { put(p, 'totalIncome', v.totalIncome); put(p, 'impureAmount', v.impureAmount); }
      else { put(p, 'dividendAmount', v.dividendAmount); put(p, 'impureRatio', v.impureRatio); }
      return p;
    },
    render(d) {
      return [
        kvRows([
          ['Purge to charity', fmtMoney(d.purgeAmount), { emphasis: true }],
          ['Clean remainder', fmtMoney(d.cleanAmount)],
        ]),
        notice('The purged amount must be given to charity — without expectation of reward.', 'info'),
      ];
    },
  },

  // --- Investment ------------------------------------------------------------
  {
    id: 'screener', group: 'Investment',
    title: 'Halal stock screener', blurb: 'AAOIFI screening: business activity + three financial ratios.',
    endpoint: '/api/v1/invest/screener',
    fields: [
      { name: 'prohibitedActivities', label: 'Prohibited activities the company engages in', type: 'multicheck',
        options: [
          { value: 'alcohol', label: 'Alcohol' }, { value: 'gambling', label: 'Gambling' },
          { value: 'pork', label: 'Pork' }, { value: 'conventional_banking', label: 'Conventional banking' },
          { value: 'conventional_insurance', label: 'Conventional insurance' },
          { value: 'adult_entertainment', label: 'Adult entertainment' }, { value: 'tobacco', label: 'Tobacco' }] },
      { name: 'interestBearingDebt', label: 'Interest-bearing debt', type: 'money' },
      { name: 'interestBearingInvestments', label: 'Cash + interest-bearing securities', type: 'money' },
      { name: 'marketCap', label: 'Market capitalization', type: 'money', placeholder: '1000000000' },
      { name: 'impureIncome', label: 'Non-compliant income', type: 'money' },
      { name: 'totalRevenue', label: 'Total revenue', type: 'money', placeholder: '200000000' },
    ],
    payload(v) {
      const p = { marketCap: v.marketCap, totalRevenue: v.totalRevenue };
      if (v.prohibitedActivities?.length) p.prohibitedActivities = v.prohibitedActivities;
      put(p, 'interestBearingDebt', v.interestBearingDebt);
      put(p, 'interestBearingInvestments', v.interestBearingInvestments);
      put(p, 'impureIncome', v.impureIncome);
      return p;
    },
    render(d) {
      const checkLabel = { debt_to_market_cap: 'Debt / market cap',
        interest_investments_to_market_cap: 'Interest investments / market cap',
        impure_income_to_revenue: 'Impure income / revenue' };
      return [
        el('div', { class: 'mb-3' },
          badge(d.verdict === 'compliant' ? '✓ COMPLIANT' : '✗ NON-COMPLIANT', d.verdict === 'compliant' ? 'green' : 'red')),
        !d.activityPassed && notice(`${t('Business activity fails the screen:')} ${(d.failedActivities || []).map(a => t(a)).join(', ')}`, 'error'),
        sectionTitle('Financial ratio screens (must stay below the threshold)'),
        dataTable(
          [{ key: 'name', label: 'Check' }, { key: 'ratio', label: 'Ratio', align: 'right' },
           { key: 'threshold', label: 'Threshold', align: 'right' }, { key: 'status', label: 'Result' }],
          d.checks.map(c => ({
            name: t(checkLabel[c.key] || c.key),
            ratio: fmtPercent(c.ratio), threshold: '< ' + fmtPercent(c.threshold),
            status: c.passed ? '✓ pass' : '✗ fail',
          }))),
        kvRows([['Purification ratio (for dividends)', fmtPercent(d.purificationRatio), { tone: 'brass' }]]),
        notice('Purify dividends from this stock via the Tazkiya calculator using this ratio.', 'info'),
      ];
    },
  },

  {
    id: 'sukuk', group: 'Investment',
    title: 'Sukuk portfolio', blurb: 'Expected income and yield for sukuk positions.',
    endpoint: '/api/v1/invest/sukuk',
    fields: [],
    list: {
      name: 'positions', label: 'Positions',
      columns: [
        { name: 'name', label: 'Name', type: 'text', placeholder: 'UzAuto 2031' },
        { name: 'faceValue', label: 'Face value', type: 'money', placeholder: '100000000' },
        { name: 'purchasePrice', label: 'Purchase price', type: 'money', placeholder: '95000000' },
        { name: 'distributionRateAnnual', label: 'Rate', type: 'money', placeholder: '0.09' },
        { name: 'frequency', label: 'Freq/yr', type: 'select',
          options: [{ value: '1', label: '1' }, { value: '2', label: '2' }, { value: '4', label: '4' }, { value: '12', label: '12' }] },
        { name: 'termMonths', label: 'Months', type: 'text', placeholder: '60' },
      ],
      defaultRows: [{ name: '', faceValue: '', purchasePrice: '', distributionRateAnnual: '', frequency: '2', termMonths: '' }],
    },
    payload(v) {
      return {
        positions: (v.positions || []).map(p => {
          const row = { faceValue: p.faceValue, purchasePrice: p.purchasePrice,
            distributionRateAnnual: p.distributionRateAnnual,
            frequency: int(p.frequency), termMonths: int(p.termMonths) };
          put(row, 'name', p.name);
          return row;
        }),
      };
    },
    render(d) {
      return [
        notGuaranteed(),
        kvRows([
          ['Total invested', fmtMoney(d.totalInvested)],
          ['Total face value', fmtMoney(d.totalFace)],
          ['Expected annual income', fmtMoney(d.totalAnnualIncome), { emphasis: true }],
          ['Portfolio current yield', fmtPercent(d.portfolioCurrentYield), { tone: 'brass' }],
          ['Total expected gain (incl. redemption)', fmtMoney(d.totalExpectedGain), { emphasis: true }],
        ]),
        sectionTitle('Per position'),
        dataTable(
          [{ key: 'name', label: 'Name' }, { key: 'payments', label: 'Payments', align: 'right' },
           { key: 'periodic', label: 'Per period', align: 'right' }, { key: 'total', label: 'Distributions', align: 'right' },
           { key: 'gain', label: 'Expected gain', align: 'right' }, { key: 'yield', label: 'Yield', align: 'right' }],
          d.positions.map(p => ({
            name: p.name || '—', payments: String(p.payments),
            periodic: fmtMoney(p.periodicDistribution), total: fmtMoney(p.totalExpectedDistributions),
            gain: fmtMoney(p.expectedGain), yield: fmtPercent(p.currentYield),
          }))),
      ];
    },
  },
];

export const calcById = id => CALCS.find(c => c.id === id);
