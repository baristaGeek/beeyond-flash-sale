import { useQuery } from "@tanstack/react-query";
import { listSales } from "@/api/client";

export const SALES_QUERY_KEY = ["sales"] as const;

export function useSalesList() {
  return useQuery({
    queryKey: SALES_QUERY_KEY,
    queryFn: listSales,
    refetchInterval: 1000,
    staleTime: 0,
  });
}
