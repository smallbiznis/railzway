export const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
export const API_KEY = __ENV.API_KEY || 'vk_live_key_F8Z7GH40KA2O_e466653d93c88393d827eccc01277ba4d59bc4c43fa42f014937fb936d3ff6e9';
export const ORG_ID = __ENV.ORG_ID || '2007115164859502592';

export const CUSTOMER_ID = __ENV.CUSTOMER_ID || '2007146434607976448';
export const METER_CODE = __ENV.METER_CODE || 'meter-usage-1767375975478-adylf7';

export const ADMIN_SESSION = __ENV.ADMIN_SESSION || '';

export const PAYMENT_PROVIDER = __ENV.PAYMENT_PROVIDER || '';
export const WEBHOOK_PAYLOAD = __ENV.WEBHOOK_PAYLOAD || '';
export const WEBHOOK_HEADERS = __ENV.WEBHOOK_HEADERS || '';

export const AUTH_COMPARE = String(__ENV.AUTH_COMPARE || '').toLowerCase() === 'true';
