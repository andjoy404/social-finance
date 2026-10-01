import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import '../../data/api_warga_models.dart';
import '../../data/warga_repository.dart';
import '../../data/warga_providers.dart';

/// Screen for editing an existing household (warga biasa).
class HouseholdEditScreen extends ConsumerStatefulWidget {
  final String householdId;

  const HouseholdEditScreen({super.key, required this.householdId});

  @override
  ConsumerState<HouseholdEditScreen> createState() =>
      _HouseholdEditScreenState();
}

class _HouseholdEditScreenState extends ConsumerState<HouseholdEditScreen> {
  final _formKey = GlobalKey<FormState>();
  final _houseNumberCtrl = TextEditingController();
  final _headNameCtrl = TextEditingController();
  final _nikCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();
  final _addressCtrl = TextEditingController();

  String _occupancyStatus = 'OWNER';
  bool _isActive = true;
  String? _startDate;
  bool _loading = true;
  bool _saving = false;
  String? _error;
  List<String> _validationErrors = [];
  bool _notFound = false;

  @override
  void initState() {
    super.initState();
    _nikCtrl.addListener(() {
      if (_nikCtrl.text.length > 16) {
        _nikCtrl.text = _nikCtrl.text.substring(0, 16);
        _nikCtrl.selection = TextSelection.collapsed(
          offset: _nikCtrl.text.length,
        );
      }
    });
    _loadHousehold();
  }

  @override
  void dispose() {
    _houseNumberCtrl.dispose();
    _headNameCtrl.dispose();
    _nikCtrl.dispose();
    _phoneCtrl.dispose();
    _emailCtrl.dispose();
    _addressCtrl.dispose();
    super.dispose();
  }

  Future<void> _loadHousehold() async {
    setState(() {
      _loading = true;
      _error = null;
    });

    try {
      final repository = ref.read(wargaRepositoryProvider);

      final response = await repository.fetchResidentsWithHouseholds(
        page: 1,
        pageSize: 100,
      );

      final resident = response.data.firstWhere(
        (r) => r.householdId == widget.householdId,
        orElse: () => throw const FormatException('Household not found'),
      );

      if (!mounted) return;

      // Find the household to get startDate
      BackendHousehold? hh;
      try {
        final households = await repository.fetchHouseholds();
        hh = households[widget.householdId];
      } catch (_) {
        // Silently ignore - startDate is optional display
      }

      setState(() {
        _houseNumberCtrl.text = resident.houseNumber;
        _headNameCtrl.text = resident.name;
        _occupancyStatus = resident.occupancyStatus == 'Pemilik'
            ? 'OWNER'
            : 'TENANT';
        _addressCtrl.text = resident.address ?? '';
        _nikCtrl.text = resident.nik ?? '';
        _phoneCtrl.text = resident.phoneNumber ?? '';
        _emailCtrl.text = resident.email ?? '';
        _isActive = resident.isActive;
        _startDate = hh?.startDate;
        _loading = false;
      });
    } on FormatException catch (_) {
      if (mounted) {
        setState(() {
          _notFound = true;
          _loading = false;
        });
      }
    } catch (e) {
      final error = ref.read(wargaRepositoryProvider).mapDioError(e);
      if (mounted) {
        setState(() {
          _error = error.message;
          _loading = false;
        });
      }
    }
  }

  bool _validate() {
    final errors = <String>[];

    if (_houseNumberCtrl.text.trim().isEmpty) {
      errors.add('Nomor rumah harus diisi.');
    }
    if (_headNameCtrl.text.trim().isEmpty) {
      errors.add('Nama kepala keluarga harus diisi.');
    }
    if (_nikCtrl.text.trim().isEmpty) {
      errors.add('NIK harus diisi.');
    } else if (!RegExp(r'^[0-9]{16}$').hasMatch(_nikCtrl.text.trim())) {
      errors.add('NIK harus berupa 16 digit angka.');
    }
    if (_phoneCtrl.text.trim().isEmpty) {
      errors.add('Nomor telepon harus diisi.');
    }
    if (_emailCtrl.text.trim().isEmpty) {
      errors.add('Email harus diisi.');
    } else if (!RegExp(
      r'^[^\s@]+@[^\s@]+\.[^\s@]+$',
    ).hasMatch(_emailCtrl.text.trim())) {
      errors.add('Format email tidak valid.');
    }

    setState(() => _validationErrors = errors);
    return errors.isEmpty;
  }

  Future<void> _save() async {
    if (!_validate()) return;

    setState(() {
      _saving = true;
      _error = null;
    });

    try {
      final request = UpdateHouseholdRequest(
        houseNumber: _houseNumberCtrl.text.trim().isEmpty
            ? null
            : _houseNumberCtrl.text.trim(),
        headName: _headNameCtrl.text.trim().isEmpty
            ? null
            : _headNameCtrl.text.trim(),
        nik: _nikCtrl.text.trim().isEmpty ? null : _nikCtrl.text.trim(),
        phone: _phoneCtrl.text.trim().isEmpty ? null : _phoneCtrl.text.trim(),
        email: _emailCtrl.text.trim().isEmpty ? null : _emailCtrl.text.trim(),
        occupancyStatus: _occupancyStatus.isEmpty ? null : _occupancyStatus,
        address: _addressCtrl.text.trim().isEmpty
            ? null
            : _addressCtrl.text.trim(),
        isActive: _isActive,
      );

      final repo = ref.read(wargaRepositoryProvider);
      await repo.updateHousehold(widget.householdId, request: request);

      // Refresh warga list
      await ref.read(wargaListProvider.notifier).refresh();

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Data warga berhasil diperbarui.'),
            backgroundColor: AppColors.success,
          ),
        );
        context.pop();
      }
    } catch (e) {
      final error = ref.read(wargaRepositoryProvider).mapDioError(e);
      if (mounted) {
        setState(() => _error = error.message);
      }
    } finally {
      if (mounted) setState(() => _saving = false);
    }
  }

  /// Show confirmation dialog before canceling the form.
  Future<void> _confirmCancel() async {
    final shouldCancel = await showDialog<bool>(
      context: context,
      builder: (dialogContext) => AlertDialog(
        title: const Text('Apakah Anda yakin ingin membatalkan?'),
        content: const Text('Perubahan yang belum disimpan akan hilang.'),
        actions: [
          Align(
            alignment: Alignment.center,
            child: SizedBox(
              height: 48,
              child: OutlinedButton(
                onPressed: () => Navigator.of(dialogContext).pop(false),
                style: OutlinedButton.styleFrom(
                  side: const BorderSide(color: AppColors.accent),
                  backgroundColor: AppColors.accentSoftColor(context),
                  shape: RoundedRectangleBorder(
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                ),
                child: const Text('Tidak, Tetap di Halaman'),
              ),
            ),
          ),
          const SizedBox(height: 8),
          Align(
            alignment: Alignment.center,
            child: Container(
              decoration: BoxDecoration(
                boxShadow: [
                  BoxShadow(
                    color: AppColors.danger.withValues(alpha: 0.15),
                    blurRadius: 8,
                    offset: const Offset(0, 0),
                  ),
                ],
              ),
              child: SizedBox(
                height: 48,
                child: OutlinedButton(
                  onPressed: () => Navigator.of(dialogContext).pop(true),
                  style: OutlinedButton.styleFrom(
                    side: BorderSide(color: AppColors.danger),
                    backgroundColor:
                        Theme.of(dialogContext).brightness == Brightness.dark
                        ? AppColors.darkDangerSoft
                        : AppColors.danger.withValues(alpha: 0.20),
                    foregroundColor: AppColors.danger,
                    shape: RoundedRectangleBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                  ),
                  child: const Text('Ya, Batalkan'),
                ),
              ),
            ),
          ),
        ],
      ),
    );

    if (shouldCancel == true && mounted) {
      context.pop();
    }
  }

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    if (_loading) {
      return Scaffold(
        appBar: AppBar(title: const Text('Edit Warga')),
        body: const Center(child: CircularProgressIndicator()),
      );
    }

    if (_notFound) {
      return Scaffold(
        appBar: AppBar(title: const Text('Edit Warga')),
        body: Center(
          child: Column(
            mainAxisSize: MainAxisSize.min,
            children: [
              Icon(
                Icons.error_outline,
                size: 64,
                color: theme.colorScheme.error.withValues(alpha: 0.5),
              ),
              const SizedBox(height: 16),
              Text(
                'Data warga tidak ditemukan.',
                style: theme.textTheme.titleLarge,
              ),
              const SizedBox(height: 8),
              ElevatedButton(
                onPressed: () => Navigator.of(context).pop(),
                child: const Text('Kembali'),
              ),
            ],
          ),
        ),
      );
    }

    return PopScope(
      canPop: false,
      onPopInvokedWithResult: (didPop, result) async {
        if (didPop) return;
        await _confirmCancel();
      },
      child: Scaffold(
        appBar: AppBar(
          title: const Text('Edit Warga'),
          leading: IconButton(
            icon: const Icon(Icons.arrow_back),
            onPressed: () => _confirmCancel(),
          ),
        ),
        body: Form(
          key: _formKey,
          child: ListView(
            padding: const EdgeInsets.all(AppSpacing.base),
            children: [
              // Error message
              if (_error != null)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(
                      alpha: isDark ? 0.2 : 0.1,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                    border: Border.all(
                      color: AppColors.danger.withValues(
                        alpha: isDark ? 0.4 : 0.2,
                      ),
                    ),
                  ),
                  child: Text(
                    _error!,
                    style: theme.textTheme.bodyMedium?.copyWith(
                      color: AppColors.danger,
                    ),
                  ),
                ),
              if (_error != null) const SizedBox(height: AppSpacing.md),

              // Validation errors
              if (_validationErrors.isNotEmpty)
                Container(
                  padding: const EdgeInsets.all(AppSpacing.md),
                  decoration: BoxDecoration(
                    color: AppColors.danger.withValues(
                      alpha: isDark ? 0.15 : 0.08,
                    ),
                    borderRadius: BorderRadius.circular(AppRadius.base),
                  ),
                  child: Column(
                    crossAxisAlignment: CrossAxisAlignment.start,
                    children: [
                      Text(
                        'Data belum lengkap:',
                        style: theme.textTheme.bodyMedium?.copyWith(
                          fontWeight: FontWeight.w600,
                          color: AppColors.danger,
                        ),
                      ),
                      const SizedBox(height: AppSpacing.sm),
                      ..._validationErrors.map(
                        (e) => Padding(
                          padding: const EdgeInsets.only(bottom: 4),
                          child: Row(
                            crossAxisAlignment: CrossAxisAlignment.start,
                            children: [
                              const Text('• ', style: TextStyle(fontSize: 16)),
                              Expanded(child: Text(e)),
                            ],
                          ),
                        ),
                      ),
                    ],
                  ),
                ),

              // Nomor Rumah + Status Hunian
              Row(
                children: [
                  Expanded(
                    flex: 2,
                    child: AppTextField(
                      controller: _houseNumberCtrl,
                      label: 'Nomor Rumah',
                      hintText: 'Contoh: 001',
                      keyboardType: TextInputType.text,
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    flex: 3,
                    child: Column(
                      crossAxisAlignment: CrossAxisAlignment.start,
                      children: [
                        Text(
                          'Status Hunian',
                          style: theme.textTheme.titleMedium?.copyWith(
                            fontWeight: FontWeight.w600,
                          ),
                        ),
                        const SizedBox(height: AppSpacing.sm),
                        SegmentedButton<String>(
                          segments: [
                            ButtonSegment(
                              value: 'OWNER',
                              label: const Text(
                                'Pemilik',
                                style: TextStyle(fontSize: 12),
                              ),
                              icon: const Icon(Icons.home, size: 16),
                            ),
                            ButtonSegment(
                              value: 'TENANT',
                              label: const Text(
                                'Penyewa',
                                style: TextStyle(fontSize: 12),
                              ),
                              icon: const Icon(Icons.hail, size: 16),
                            ),
                          ],
                          selected: {_occupancyStatus},
                          onSelectionChanged: (selected) {
                            setState(() => _occupancyStatus = selected.first);
                          },
                          style: SegmentedButton.styleFrom(
                            padding: const EdgeInsets.symmetric(
                              horizontal: 8,
                              vertical: 4,
                            ),
                            backgroundColor: isDark
                                ? AppColors.darkSurface
                                : AppColors.lightSurface,
                            foregroundColor: isDark
                                ? AppColors.darkText
                                : AppColors.lightText,
                            selectedForegroundColor: AppColors.accent,
                            selectedBackgroundColor: AppColors.accentSoftColor(
                              context,
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Nama Kepala Keluarga
              AppTextField(
                controller: _headNameCtrl,
                label: 'Nama Kepala Keluarga',
                hintText: 'Nama lengkap',
              ),
              const SizedBox(height: AppSpacing.lg),

              // NIK
              AppTextField(
                controller: _nikCtrl,
                label: 'NIK',
                hintText: '16 digit',
                keyboardType: TextInputType.number,
                inputFormatters: [
                  FilteringTextInputFormatter.digitsOnly,
                  LengthLimitingTextInputFormatter(16),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Nomor Telepon
              AppTextField(
                controller: _phoneCtrl,
                label: 'Nomor Telepon',
                hintText: '08123456789',
                keyboardType: TextInputType.phone,
              ),
              const SizedBox(height: AppSpacing.lg),

              // Email
              AppTextField(
                controller: _emailCtrl,
                label: 'Email',
                hintText: 'nama@email.com',
                keyboardType: TextInputType.emailAddress,
              ),
              const SizedBox(height: AppSpacing.lg),

              // Alamat (optional)
              AppTextField(
                controller: _addressCtrl,
                label: 'Alamat (opsional)',
                hintText: 'Jl. Mawar No. 1',
              ),
              const SizedBox(height: AppSpacing.lg),

              // Tanggal Mulai Hunian (READ-ONLY display)
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Tanggal Mulai Hunian',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  Container(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    decoration: BoxDecoration(
                      border: Border.all(color: AppColors.lightBorder),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      color: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurface.withValues(alpha: 0.5),
                    ),
                    child: Row(
                      children: [
                        Icon(
                          Icons.calendar_today,
                          size: 20,
                          color: AppColors.accent,
                        ),
                        const SizedBox(width: AppSpacing.md),
                        Text(
                          _startDate ?? '-',
                          style: theme.textTheme.bodyLarge?.copyWith(
                            color: isDark
                                ? AppColors.darkTextMuted
                                : AppColors.lightTextMuted,
                          ),
                        ),
                      ],
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.lg),

              // Status Aktif/Tidak Aktif
              Column(
                crossAxisAlignment: CrossAxisAlignment.start,
                children: [
                  Text(
                    'Status',
                    style: theme.textTheme.titleMedium?.copyWith(
                      fontWeight: FontWeight.w600,
                    ),
                  ),
                  const SizedBox(height: AppSpacing.sm),
                  SegmentedButton<bool>(
                    segments: [
                      ButtonSegment(
                        value: true,
                        label: const Text('Aktif'),
                        icon: const Icon(Icons.check_circle, size: 18),
                      ),
                      ButtonSegment(
                        value: false,
                        label: const Text('Tidak Aktif'),
                        icon: const Icon(Icons.cancel, size: 18),
                      ),
                    ],
                    selected: {_isActive},
                    onSelectionChanged: (selected) {
                      setState(() => _isActive = selected.first);
                    },
                    style: SegmentedButton.styleFrom(
                      backgroundColor: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurface,
                      foregroundColor: isDark
                          ? AppColors.darkText
                          : AppColors.lightText,
                      selectedForegroundColor: AppColors.accent,
                      selectedBackgroundColor: AppColors.accentSoftColor(
                        context,
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.xxl),

              // Horizontal buttons: Simpan + Batal
              Row(
                children: [
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.accent.withValues(alpha: 0.15),
                            blurRadius: 8,
                            offset: const Offset(0, 0),
                          ),
                        ],
                      ),
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _save,
                          style: OutlinedButton.styleFrom(
                            side: const BorderSide(color: AppColors.accent),
                            foregroundColor: AppColors.accent,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),
                          child: _saving
                              ? SizedBox(
                                  width: 16,
                                  height: 16,
                                  child: CircularProgressIndicator(
                                    strokeWidth: 2,
                                    backgroundColor: Colors.transparent,
                                    valueColor: AlwaysStoppedAnimation<Color>(
                                      AppColors.accent,
                                    ),
                                  ),
                                )
                              : const Row(
                                  mainAxisSize: MainAxisSize.min,
                                  mainAxisAlignment: MainAxisAlignment.center,
                                  children: [
                                    Icon(Icons.save_outlined, size: 18),
                                    SizedBox(width: 6),
                                    Flexible(
                                      child: Text(
                                        'Simpan Perubahan',
                                        overflow: TextOverflow.ellipsis,
                                      ),
                                    ),
                                  ],
                                ),
                        ),
                      ),
                    ),
                  ),
                  const SizedBox(width: AppSpacing.md),
                  Expanded(
                    child: Container(
                      decoration: BoxDecoration(
                        boxShadow: [
                          BoxShadow(
                            color: AppColors.danger.withValues(alpha: 0.15),
                            blurRadius: 8,
                            offset: const Offset(0, 0),
                          ),
                        ],
                      ),
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _confirmCancel,
                          style: OutlinedButton.styleFrom(
                            side: BorderSide(color: AppColors.danger),
                            foregroundColor: AppColors.danger,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(
                                AppRadius.base,
                              ),
                            ),
                          ),
                          child: const Row(
                            mainAxisAlignment: MainAxisAlignment.center,
                            children: [
                              Icon(Icons.close, size: 18),
                              SizedBox(width: 6),
                              Text('Batal'),
                            ],
                          ),
                        ),
                      ),
                    ),
                  ),
                ],
              ),
              const SizedBox(height: AppSpacing.base),
            ],
          ),
        ),
      ),
    );
  }
}
