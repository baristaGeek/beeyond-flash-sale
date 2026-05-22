import { InventoryPage } from "./pages/InventoryPage";
import { ToastProvider } from "@/hooks/useToast";
import { ToastRegion } from "@/components/Toast";

export default function App() {
  return (
    <ToastProvider>
      <ToastRegion />
      <main>
        <InventoryPage />
      </main>
    </ToastProvider>
  );
}
