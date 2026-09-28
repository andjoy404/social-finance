import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:flutter_riverpod/flutter_riverpod.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/primary_button.dart';
import '../../data/api_warga_models.dart';
import '../../data/warga_repository.dart';
import '../../../../core/models/role.dart';

/// Screen for creating a new household (adding a warga/head of family).
class HouseholdCreateScreen extends ConsumerStatefulWidget {
  final bool isSuperAdmin;

  const HouseholdCreateScreen({super.key, required this.isSuperAdmin});

  @override
  ConsumerState<HouseholdCreateScreen> createState() =>
      _HouseholdCreateScreenState();
}

class _HouseholdCreateScreenState extends ConsumerState<HouseholdCreateScreen> {
  final _formKey = GlobalKey<FormState>();
  final _houseNumberCtrl = TextEditingController();
  final _headNameCtrl = TextEditingController();
  final _nikCtrl = TextEditingController();
  final _phoneCtrl = TextEditingController();
  final _emailCtrl = TextEditingController();
  final _addressCtrl = TextEditingController();

  String _occupancyStatus = 'OWNER';
  String _startDate = '';
  String? _selectedRtId;
  bool _saving = false;
  List<Map<String, dynamic>> _rtOptions = [];
  bool _rtLoading = false;
  String? _error;
  List<String> _validationErrors = [];

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

  Future<void> _loadRts() async {
    if (!widget.isSuperAdmin) return;

    setState(() => _rtLoading = true);
    try {
      final repo = ref.read(wargaRepositoryProvider);
      final rts = await repo.fetchRts();
      if (mounted) {
        setState(() => _rtOptions = rts);
      }
    } catch (e) {
      // Silently fail — RT selection not critical for non-superadmin
    } finally {
      if (mounted) setState(() => _rtLoading = false);
    }
  }

  @override
  void didChangeDependencies() {
    super.didChangeDependencies();
    _loadRts();
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
    } else if (!RegExp(r'^[^\s@]+@[^\s@]+\.[^\s@]+$').hasMatch(
      _emailCtrl.text.trim(),
    )) {
      errors.add('Format email tidak valid.');
    }
    if (_startDate.isEmpty) {
      errors.add('Tanggal mulai hunian harus diisi.');
    }
    if (widget.isSuperAdmin && _selectedRtId == null) {
      errors.add('RT Tujuan harus dipilih.');
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
      final repo = ref.read(wargaRepositoryProvider);
      final request = CreateHouseholdRequest(
        houseNumber: _houseNumberCtrl.text.trim(),
        headName: _headNameCtrl.text.trim(),
        nik: _nikCtrl.text.trim(),
        phone: _phoneCtrl.text.trim(),
        email: _emailCtrl.text.trim(),
        occupancyStatus: _occupancyStatus,
        startDate: _startDate,
        address: _addressCtrl.text.trim().isEmpty ? null : _addressCtrl.text.trim(),
      );

      await repo.createHousehold(
        request: request,
        rtId: widget.isSuperAdmin ? _selectedRtId : null,
      );

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Data warga berhasil ditambahkan.'),
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

  @override
  Widget build(BuildContext context) {
    final theme = Theme.of(context);
    final isDark = theme.brightness == Brightness.dark;

    return Scaffold(
      appBar: AppBar(
        title: const Text('Tambah Warga'),
        leading: IconButton(
          icon: const Icon(Icons.arrow_back),
          onPressed: () => context.pop(),
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
                  color: AppColors.danger.withValues(alpha: isDark ? 0.2 : 0.1),
                  borderRadius: BorderRadius.circular(AppRadius.base),
                  border: Border.all(
                    color: AppColors.danger.withValues(alpha: isDark ? 0.4 : 0.2),
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
                  color: AppColors.danger.withValues(alpha: isDark ? 0.15 : 0.08),
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
                    ..._validationErrors.map((e) => Padding(
                      padding: const EdgeInsets.only(bottom: 4),
                      child: Row(
                        crossAxisAlignment: CrossAxisAlignment.start,
                        children: [
                          const Text('• ', style: TextStyle(fontSize: 16)),
                          Expanded(child: Text(e)),
                        ],
                      ),
                    )),
                  ],
                ),
              ),

            // RT selector for superadmin
            if (widget.isSuperAdmin) ...[
              const SizedBox(height: AppSpacing.lg),
              Text(
                'RT Tujuan',
                style: theme.textTheme.titleMedium?.copyWith(
                  fontWeight: FontWeight.w600,
                ),
              ),
              const SizedBox(height: AppSpacing.sm),
              if (_rtLoading)
                const Center(child: CircularProgressIndicator())
              else if (_rtOptions.isEmpty)
                Text(
                  'Tidak ada RT yang tersedia.',
                  style: theme.textTheme.bodyMedium?.copyWith(
                    color: theme.colorScheme.onSurfaceVariant,
                  ),
                )
              else
                DropdownButtonFormField<String>(
                  decoration: InputDecoration(
                    labelText: 'Pilih RT',
                    border: OutlineInputBorder(
                      borderRadius: BorderRadius.circular(AppRadius.base),
                    ),
                    contentPadding: const EdgeInsets.symmetric(
                      horizontal: AppSpacing.md,
                      vertical: AppSpacing.sm,
                    ),
                  ),
                  value: _selectedRtId,
                  hint: const Text('Pilih RT'),
                  items: _rtOptions.map((rt) {
                    final rtNum = ((rt['rt'] as String?) ?? '').padLeft(3, '0');
                    final rwNum = ((rt['rw'] as num?)?.toInt().toString().padLeft(3, '0')) ?? '';
                    final name = (rt['name'] as String?) ?? 'RT tanpa nama';
                    return DropdownMenuItem(
                      value: rt['id'] as String?,
                      child: Text('RT $rtNum / RW $rwNum — $name'),
                    );
                  }).toList(),
                  onChanged: widget.isSuperAdmin
                      ? (value) => setState(() => _selectedRtId = value)
                      : null,
                  validator: (value) {
                    if (widget.isSuperAdmin && (value == null || value.isEmpty)) {
                      return 'RT Tujuan harus dipilih.';
                    }
                    return null;
                  },
                ),
              const SizedBox(height: AppSpacing.lg),
            ],

            // Nomor Rumah + Status Hunian
            Row(
              children: [
                Expanded(
                  flex: 2,
                  child: AppTextField(
                    controller: _houseNumberCtrl,
                    label: 'Nomor Rumah',
                    hintText: 'Contoh: 001',
                    keyboardType: TextInputType.number,
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
                            label: const Text('Pemilik'),
                            icon: const Icon(Icons.home, size: 18),
                          ),
                          ButtonSegment(
                            value: 'TENANT',
                            label: const Text('Penyewa'),
                            icon: const Icon(Icons.hail, size: 18),
                          ),
                        ],
                        selected: {_occupancyStatus},
                        onSelectionChanged: (selected) {
                          setState(() => _occupancyStatus = selected.first);
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

            // NIK + Nomor Telepon
            Row(
              children: [
                Expanded(
                  flex: 2,
                  child: AppTextField(
                    controller: _nikCtrl,
                    label: 'NIK',
                    hintText: '16 digit',
                    keyboardType: TextInputType.number,
                    inputFormatters: [
                      FilteringTextInputFormatter.digitsOnly,
                      LengthLimitingTextInputFormatter(16),
                    ],
                  ),
                ),
                const SizedBox(width: AppSpacing.md),
                Expanded(
                  flex: 3,
                  child: AppTextField(
                    controller: _phoneCtrl,
                    label: 'Nomor Telepon',
                    hintText: '08123456789',
                    keyboardType: TextInputType.phone,
                  ),
                ),
              ],
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

            // Tanggal Mulai Hunian
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
                InkWell(
                  onTap: () async {
                    final picked = await showDatePicker(
                      context: context,
                      initialDate: DateTime.now(),
                      firstDate: DateTime(2000),
                      lastDate: DateTime.now().add(
                        const Duration(days: 365),
                      ),
                    );
                    if (picked != null) {
                      setState(() {
                        _startDate = picked.toLocal().toString().split(' ')[0];
                      });
                    }
                  },
                  child: Container(
                    padding: const EdgeInsets.all(AppSpacing.md),
                    decoration: BoxDecoration(
                      border: Border.all(
                        color: AppColors.lightBorder,
                      ),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      color: isDark
                          ? AppColors.darkSurface
                          : AppColors.lightSurface,
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
                          _startDate.isEmpty
                              ? 'Pilih tanggal'
                              : _startDate,
                          style: theme.textTheme.bodyLarge,
                        ),
                        const Spacer(),
                        const Icon(
                          Icons.arrow_forward_ios,
                          size: 16,
                          color: Colors.grey,
                        ),
                      ],
                    ),
                  ),
                ),
              ],
            ),
            const SizedBox(height: AppSpacing.lg),

            // Alamat (optional)
            AppTextField(
              controller: _addressCtrl,
              label: 'Alamat (opsional)',
              hintText: 'Jl. Mawar No. 1',
            ),
            const SizedBox(height: AppSpacing.xxl),

            // Save button
            SizedBox(
              width: double.infinity,
              child: PrimaryButton(
                text: _saving ? 'Menyimpan...' : 'Simpan',
                onPressed: _save,
                isLoading: _saving,
              ),
            ),
            const SizedBox(height: AppSpacing.base),
          ],
        ),
      ),
    );
  }
}
