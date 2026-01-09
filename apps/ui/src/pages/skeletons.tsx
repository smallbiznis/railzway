import {
  CustomersTableSkeleton,
  ProductsTableSkeleton,
  ReportsPageSkeleton,
  SettingsFormSkeleton,
} from "@/components/loading-skeletons"

export default function SkeletonExamplesPage() {
  return (
    <div className="space-y-12">
      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Products list loading</h2>
        <ProductsTableSkeleton />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Customers list loading</h2>
        <CustomersTableSkeleton />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Reports page loading</h2>
        <ReportsPageSkeleton />
      </section>

      <section className="space-y-3">
        <h2 className="text-lg font-semibold">Settings page loading</h2>
        <SettingsFormSkeleton />
      </section>
    </div>
  )
}
