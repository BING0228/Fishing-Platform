"use client";

export function DemoBanner() {
  return (
    <div className="pointer-events-none fixed bottom-4 right-4 z-[100] rounded-md border border-amber-300 bg-amber-50 px-3 py-2 text-xs font-medium text-amber-900 shadow-lg">
      Static demo · mock data · read-only
    </div>
  );
}
