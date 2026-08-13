DELETE FROM exchange_rates
WHERE (base_currency = 'USD' AND target_currency IN ('EUR', 'RUB'))
   OR (base_currency = 'EUR' AND target_currency IN ('USD', 'RUB'))
   OR (base_currency = 'RUB' AND target_currency IN ('USD', 'EUR'));