INSERT INTO payment_provider_catalog (provider, display_name, description, supports_webhook, supports_refund)
VALUES
  ('adyen', 'Adyen', 'Global payments platform.', TRUE, TRUE),
  ('braintree', 'Braintree', 'PayPal service for cards and wallets.', TRUE, TRUE)
ON CONFLICT (provider) DO NOTHING;
