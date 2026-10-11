import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom";
import type { ComponentType } from "react";
import { AuthProvider } from "./auth/auth";
import { RequireAuth } from "./auth/RequireAuth";
import { PlatformProvider } from "./store/platform";
import { AppShell } from "./components/AppShell";

import LandingPage from "./pages/LandingPage";
import AuthPage from "./pages/AuthPage";
import DashboardPage from "./pages/DashboardPage";
import FrontDeskPage from "./pages/FrontDeskPage";
import ShiftHandoverPage from "./pages/ShiftHandoverPage";
import RoomsPage from "./pages/RoomsPage";
import ReservationsPage from "./pages/ReservationsPage";
import HousekeepingPage from "./pages/HousekeepingPage";
import MaintenancePage from "./pages/MaintenancePage";
import YieldManagementPage from "./pages/YieldManagementPage";
import NightAuditPage from "./pages/NightAuditPage";
import PosPage from "./pages/PosPage";
import InventoryPage from "./pages/InventoryPage";
import FinancePage from "./pages/FinancePage";
import HrPage from "./pages/HrPage";
import CrmPage from "./pages/CrmPage";
import ChannelsPage from "./pages/ChannelsPage";
import PaymentsPage from "./pages/PaymentsPage";
import EventsPage from "./pages/EventsPage";
import HardwarePage from "./pages/HardwarePage";
import ReportsPage from "./pages/ReportsPage";
import AdminPage from "./pages/AdminPage";

// Public booking website (M-LOD / M-DST integration)
import BookPage from "./pages/booking/BookPage";
import GuestDetailsPage from "./pages/booking/GuestDetailsPage";
import ConfirmationPage from "./pages/booking/ConfirmationPage";
import BookingLookupPage from "./pages/booking/BookingLookupPage";

// Operator marketing site (B2B)
import PlatformPage from "./pages/platform/PlatformPage";

const protectedRoutes: Array<[string, ComponentType]> = [
  ["/dashboard", DashboardPage],
  ["/front-desk", FrontDeskPage],
  ["/shift-handover", ShiftHandoverPage],
  ["/rooms", RoomsPage],
  ["/reservations", ReservationsPage],
  ["/housekeeping", HousekeepingPage],
  ["/maintenance", MaintenancePage],
  ["/yield-management", YieldManagementPage],
  ["/night-audit", NightAuditPage],
  ["/pos", PosPage],
  ["/inventory", InventoryPage],
  ["/finance", FinancePage],
  ["/hr", HrPage],
  ["/crm", CrmPage],
  ["/channels", ChannelsPage],
  ["/payments", PaymentsPage],
  ["/events", EventsPage],
  ["/hardware", HardwarePage],
  ["/reports", ReportsPage],
  ["/admin", AdminPage],
];

export default function App() {
  return (
    <BrowserRouter>
      <AuthProvider>
        <PlatformProvider>
          <Routes>
            {/* Public product surfaces */}
            <Route path="/" element={<LandingPage />} />
            <Route path="/platform" element={<PlatformPage />} />
            <Route path="/auth" element={<AuthPage />} />
            <Route path="/book" element={<BookPage />} />
            <Route path="/book/guests" element={<GuestDetailsPage />} />
            <Route path="/booking/confirmation" element={<ConfirmationPage />} />
            <Route path="/booking/lookup" element={<BookingLookupPage />} />

            {/* Staff console (protected) */}
            <Route element={<RequireAuth><AppShell /></RequireAuth>}>
              {protectedRoutes.map(([path, Page]) => (
                <Route key={path} path={path} element={<Page />} />
              ))}
            </Route>
            <Route path="*" element={<Navigate to="/" replace />} />
          </Routes>
        </PlatformProvider>
      </AuthProvider>
    </BrowserRouter>
  );
}
