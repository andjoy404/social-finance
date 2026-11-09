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
import 'rw_notification_screen.dart';

/// Screen gabungan Keuangan / DATA RW dengan Tab: BERANDA, KAS, dan LOKASI DANA.
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
    with TickerProviderStateMixin {
  TabController? _tabController;

  @override
  void initState() {
    super.initState();
    _tabController = TabController(
      length: 2,
      vsync: this,
      initialIndex: 0,
    );
  }

  @override
  void didUpdateWidget(covariant RwKeuanganScreen oldWidget) {
    super.didUpdateWidget(oldWidget);
    if (oldWidget.initialTab != widget.initialTab) {
      final validIndex = widget.initialTab.clamp(0, _tabController!.length - 1);
      if (_tabController!.index != validIndex) {
        _tabController!.index = validIndex;
      }
    }
  }

  @override
  void dispose() {
    _tabController?.dispose();
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

    // Final Matrix:
    // Pengurus RT (rt_inferred): Beranda RW only. KAS/Lokasi Dana/Notifikasi NO.
    // Pengurus RW (explicit): Beranda, KAS, Lokasi Dana, Notifikasi (read; write by jabatan).
    // Ketua RW & Bendahara RW explicit: write-capable per feature/backend.
    // Super Admin: all tabs.
    final isSuperAdmin = user?['role'] == 'super_admin' || user?['system_role'] == 'super_admin';
    final isExplicitMember = currentMembership?.source == 'explicit';
    final hasFeatureAccess = rwMembershipHasFeatureAccess(user, widget.rwId);
    final showKas = hasFeatureAccess;
    final showLokasiDana = hasFeatureAccess;
    final showNotifikasi = isSuperAdmin || isExplicitMember; // explicit read + SA; rt_inferred hidden
    final tabCount = 1 + (showKas ? 1 : 0) + (showLokasiDana ? 1 : 0) + (showNotifikasi ? 1 : 0);

    // Recreate controller if tab count changed (e.g. inferred → explicit or
    // vice versa). TabController.length has no setter, so we must dispose
    // and allocate a new controller rather than mutating in place.
    if (_tabController == null || _tabController!.length != tabCount) {
      _tabController?.dispose();
      _tabController = TabController(
        length: tabCount,
        vsync: this,
        initialIndex: widget.initialTab.clamp(0, tabCount - 1),
      );
    }

    // Clamp index to valid range after length change.
    final validIndex = widget.initialTab.clamp(0, tabCount - 1);
    if (_tabController!.index != validIndex) {
      _tabController!.index = validIndex;
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
            if (showKas) const Tab(text: 'KAS'),
            if (showLokasiDana) const Tab(text: 'LOKASI DANA'),
            if (showNotifikasi) const Tab(text: 'NOTIFIKASI'),
          ],
        ),
      ),
      body: TabBarView(
        controller: _tabController,
        children: [
          RwDashboardScreen(rwId: widget.rwId, isTabView: true),
          if (showKas) RwKasScreen(rwId: widget.rwId, isTabView: true),
          if (showLokasiDana)
            RwFundLocationScreen(rwId: widget.rwId, isTabView: true),
          if (showNotifikasi)
            RwNotificationScreen(rwId: widget.rwId),
        ],
      ),
    );
  }
}
