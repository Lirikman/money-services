INSERT INTO exchange_rates(base_currency,target_currency,rate)
VALUES
    ('USD','EUR',0.87),
    ('USD','RUB',81.52),
    ('EUR','USD',1.15),
    ('EUR','RUB',94.03),
    ('RUB','USD',0.012),
    ('RUB','EUR',0.011)
ON CONFLICT (base_currency, target_currency) 
DO UPDATE SET 
    rate = EXCLUDED.rate,
    updated_at = NOW();