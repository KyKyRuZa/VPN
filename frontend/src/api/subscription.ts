import api from "./axios";

export interface Subscription {
  subscription_url: string;
  username: string;
}

export async function getSubscription(): Promise<Subscription> {
  const { data } = await api.get<Subscription>("/subscription");
  return data;
}
