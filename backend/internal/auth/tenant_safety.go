// Package auth establishes the tenant-safety convention that all future
// repository packages must follow.
//
// TENANT-SAFETY CONVENTION (Phase 2)
// ==================================
//
// Every repository method operating on tenant-owned data MUST receive an
// authenticated RTID from the request handler and include it in all queries.
//
// Pattern:
//
//	func (r *Repository) FindByID(ctx context.Context, tx *sql.Tx, id, rtID string) (*Thing, error) {
//	    query := `
//	        SELECT id, rt_id, name
//	        FROM things
//	        WHERE id = $1 AND rt_id = $2
//	    `
//	    // ...
//	}
//
// Rules:
//
//  1. NEVER trust client-supplied rt_id. The RTID comes from the AuthContext
//     populated by RequireAuth middleware from the validated JWT.
//     (See: middleware.go::RequireAuth — it constructs AuthContext solely
//     from jwt.ParseAndVerifyAccessToken.  URL path params, query strings,
//     and JSON body fields are IGNORED.)
//
//  2. Every query on tenant-scoped data includes AND rt_id = ?.
//     This is a belt-and-suspenders defense.  Even if a resource ID
//     (e.g. bill_id) belongs to another RT, the rt_id filter prevents
//     cross-tenant access.
//
//  3. Resource ID alone is NEVER sufficient to identify tenant-owned data.
//     A UUID collision across tenants is expected and normal.  The
//     composite (id, rt_id) is the actual unique key for tenant-scoped lookups.
//
//  4. SUPER_ADMIN access: when a system-level endpoint operates on a
//     specific RT, the rt_id is supplied via the authenticated request path
//     (e.g. /api/v1/rt/:id/resources) and verified against the rts table
//     after being extracted from JWT claims (AuthContext.RTID is empty for
//     system-only super_admins; the explicit path parameter overrides it).
//
//  5. Do NOT implement tenant isolation in the handler or service layer
//     alone. The database query MUST include the rt_id filter.  Handler-
//     level checks are convenience only and easily bypassed.
//
// Example violation to prevent:
//
//		BAD — trusts client-supplied rt_id:
//		    func FindBill(clientRTID, billID) { ... WHERE id = $1 AND rt_id = $2 }
//
//		GOOD — rtID derived from AuthContext:
//		    func FindBill(ac *AuthContext, tx *sql.Tx, billID string) {
//		        query: ... WHERE id = $1 AND rt_id = $2
//		        params: billID, ac.RTID   // always from JWT
//		    }
//
//	 6. Foreign-key columns (e.g. user_id, category_id) should also be
//	    indexed, but they do NOT provide tenant isolation.  Only rt_id does.
//
//	 7. Soft deletes (is_active) do not replace tenant isolation.  A deleted
//	    resource may still be queryable depending on the repository
//	    implementation.  The rt_id filter applies regardless of is_active.
package auth
