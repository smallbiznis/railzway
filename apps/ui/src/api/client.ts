import axios from "axios";

import type { AxiosInstance } from "axios";

import { useOrgStore } from "@/stores/orgStore";

export const auth = axios.create({
  baseURL: "/auth",
  withCredentials: true,
});

export const admin = axios.create({
  baseURL: "/admin",
  withCredentials: true,
});

const attachOrgHeader = (client: AxiosInstance) => {
  client.interceptors.request.use((config) => {
    const orgId = useOrgStore.getState().currentOrg?.id;
    if (orgId) {
      config.headers = config.headers ?? {};
      config.headers["X-Org-Id"] = orgId;
    }
    return config;
  });
};

attachOrgHeader(admin);
