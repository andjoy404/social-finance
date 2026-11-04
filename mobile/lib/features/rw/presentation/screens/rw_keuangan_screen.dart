import 'package:flutter/material.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:social_finance/core/api/models.dart';
import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/utils/rw_utils.dart';
import 'package:social_finance/core/widgets/menu_app_bar_title.dart';
import 'package:social_finance/features/auth/data/mock_auth_repository.dart';

import 'rw_dashboard_screen.dart';
import 'rw_fund_location_screen.dart';
import 'rw_kas_screen.dart';

/// Check if a user has full RW write access for a specific RW.
///
/// Checks only the membership for the given `rwId`:
/// - Super Admin → true
/// - source === 'explicit' → true
/// - source === 'rt_inferred' or missing → false
bool _rwMembershipHasWriteAccess(
  Map<String, dynamic>? user,
  String rwId,
) {
  if (user == null) return false;
  if (user['role'] == 'super_admin' || user['system_role'] == 'super_admin') return true;

  final rawMemberships = (user['rw_memberships'] as List?)
          ?.whereType<Map<String, dynamic>>()
          .toList() ??
      <Map<String, dynamic>>[];
  final membership = rawMemberships
      .where((m) => m['rw_id'] == rwId)
      .firstOrNull as Map<String, dynamic>?;

  if (membership == null) return false;
  return membership['source'] == 'explicit';
}

/// Screen gabungan Keuangan / DATA RW dengan 3 Tab: BERANDA, KAS, dan LOKASI DANA.
class RwKeuanganScreen extends ConsumerStatefulWidget {
  final String rwId;
  final int initialTab;

  const RwKeuanganScreen({
    super.key,
    required this.rwId,
    this.initialTab = 0,
  });

  @override
  ConsumerState<RwKeuanganScreen> createState() => _RwKeuanganScreenState();
}

class _RwKeuanganScreenState extends ConsumerState<RwKeuanganScreen>
    with SingleTickerProviderStateMixin {
  late final TabController _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(
      length: 3,
      vsync: this,
      initialIndex: widget.initialTab.clamp(0, 2),
    );
  }

  @override
  void didUpdateWidget(covariant RwKeuanganScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.initialTab != widget.initialTab) {
      _tabController.animateTo(widget.initialTab.clamp(0, 2));
    }
  }

  @override
  void dispose() {
    _tabController.dispose();
    super.dispose();
  }

  @override
  Widget build(BuildContext context) {
    final authState = ref.watch(authRepositoryProvider);
    final user = authState?.valueOrNull;
    final rwMemberships = (user?['rw_memberships'] as List?)
            ?.whereType<Map<String, dynamic>>()
            .map((e) => RWMembership.fromJson(e.cast<String, dynamic>()))
            .toList() ??
        <RWMembership>[];
    final currentMembership =
        rwMemberships.where((m) => m.rwId == widget.rwId).firstOrNull;
    final rwNumberText = currentMembership?.rwNumber != null
        ? currentMembership!.rwNumber!.toString().padLeft(3, '0')
        : resolveRwNumber(widget.rwId, rwMemberships);

    // Source-aware: hide LOKASI DANA tab for rt_inferred users.
    final hasWriteAccess = _rwMembershipHasWriteAccess(user, widget.rwId);
    final showLokasiDana = hasWriteAccess;
    final tabCount = 2 + (showLokasiDana ? 1 : 0);

    // Re-init TabController if tab count changed.
    if (_tabController.length != tabCount) {
      _tabController.dispose();
      _tabController = TabController(
        length: tabCount,
        vsync: this,
        initialIndex: widget.initialTab.clamp(0, tabCount - 1),
      );
    }

    return Scaffold(
      appBar: AppBar(
        automaticallyImplyLeading: true,
        centerTitle: false,
        title: MenuAppBarTitle(title: 'Data RW', rw: rwNumberText),
        bottom: TabBar(
          controller: _tabController,
          labelColor: AppColors.accent,
          indicatorColor: AppColors.accent,
          indicatorWeight: 3,
          tabs: [
            const Tab(text: 'BERANDA'),
            const Tab(text: 'KAS'),
            if (showLokasiDana) const Tab(text: 'LOKASI DANA'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          RwDashboardScreen(rwId: widget.rwId, isTabView: true),
          RwKasScreen(rwId: widget.rwId, isTabView: true),
          if (showLokasiDana)
            RwFundLocationScreen(rwId: widget.rwId, isTabView: true),
        ],
      ),
    );
  }
}
