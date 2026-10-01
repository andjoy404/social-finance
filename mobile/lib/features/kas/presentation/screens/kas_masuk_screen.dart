import 'package:flutter/material.dart';
import 'package:flutter/services.dart';
import 'package:go_router/go_router.dart';

import 'package:social_finance/core/theme/app_colors.dart';
import 'package:social_finance/core/theme/app_radius.dart';
import 'package:social_finance/core/theme/app_spacing.dart';
import 'package:social_finance/core/utils/rupiah_formatter.dart';
import 'package:social_finance/core/widgets/app_badge.dart';
import 'package:social_finance/core/widgets/app_card.dart';
import 'package:social_finance/core/widgets/app_text_field.dart';
import 'package:social_finance/core/widgets/double_back_exit_scope.dart';
import 'package:social_finance/core/widgets/section_header.dart';
import 'package:social_finance/features/kas/data/kas_models.dart';

/// Form screen for creating a new Kas Masuk (income) transaction.
class KasMasukScreen extends StatefulWidget {
  const KasMasukScreen({super.key});

  @override
  State<KasMasukScreen> createState() => _KasMasukScreenState();
}

class _KasMasukScreenState extends State<KasMasukScreen> {
  final _formKey = GlobalKey<FormState>();
  final _tanggalCtrl = TextEditingController();
  final _nominalCtrl = TextEditingController();
  final _keteranganCtrl = TextEditingController();
  final _referensiCtrl = TextEditingController();

  String? _selectedKategori;
  bool _saving = false;
  String? _error;
  List<String> _validationErrors = [];

  static const List<String> _kategoriOptions = [
    'Iuran Warga',
    'Iuran Paksa',
    'Perlengkapan',
    'Konsumsi',
    'Perbaikan',
    'Kebersihan',
    'Keamanan',
    'Lainnya',
  ];

  @override
  void initState() {
    super.initState();
    // Default to today
    final now = DateTime.now();
    _tanggalCtrl.text =
        '${now.year}-${now.month.toString().padLeft(2, '0')}-${now.day.toString().padLeft(2, '0')}';
    // Set default kategori
    _selectedKategori = 'Iuran Warga';
  }

  @override
  void dispose() {
    _tanggalCtrl.dispose();
    _nominalCtrl.dispose();
    _keteranganCtrl.dispose();
    _referensiCtrl.dispose();
    super.dispose();
  }

  bool _validate() {
    final errors = <String>[];

    if (_tanggalCtrl.text.trim().isEmpty) {
      errors.add('Tanggal harus diisi.');
    }
    if (_selectedKategori == null || _selectedKategori!.isEmpty) {
      errors.add('Kategori harus dipilih.');
    }
    if (_nominalCtrl.text.trim().isEmpty) {
      errors.add('Nominal harus diisi.');
    } else {
      final parsed = int.tryParse(_nominalCtrl.text.trim());
      if (parsed == null || parsed <= 0) {
        errors.add('Nominal harus berupa angka yang lebih besar dari 0.');
      }
    }
    if (_keteranganCtrl.text.trim().isEmpty) {
      errors.add('Keterangan harus diisi.');
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
      final nominal = int.parse(_nominalCtrl.text.trim());

      // Map selected label to KasCategory
      KasCategory kategori;
      switch (_selectedKategori) {
        case 'Iuran Warga':
          kategori = KasCategory.iuranWarga;
          break;
        case 'Iuran Paksa':
          kategori = KasCategory.iuranPaksa;
          break;
        case 'Perlengkapan':
          kategori = KasCategory.perlengkapan;
          break;
        case 'Konsumsi':
          kategori = KasCategory.konsumsi;
          break;
        case 'Perbaikan':
          kategori = KasCategory.perbaikan;
          break;
        case 'Kebersihan':
          kategori = KasCategory.kebersihan;
          break;
        case 'Keamanan':
          kategori = KasCategory.keamanan;
          break;
        default:
          kategori = KasCategory.lainLain;
          break;
      }

      final mockTx = KasTransaction(
        id: 'kas-new-${DateTime.now().millisecondsSinceEpoch}',
        tanggal: DateTime.tryParse(_tanggalCtrl.text.trim()) ?? DateTime.now(),
        jenis: KasJenis.masuk,
        kategori: kategori,
        keterangan: _keteranganCtrl.text.trim(),
        referensi: _referensiCtrl.text.trim().isEmpty
            ? null
            : _referensiCtrl.text.trim(),
        nominal: nominal,
        saldo: nominal, // Mock running balance
      );

      // Simulate saving to mock data
      print('[KAS] Mock KasMasuk saved:');
      print('  ID: ${mockTx.id}');
      print('  Tanggal: ${formatTanggal(mockTx.tanggal)}');
      print('  Kategori: ${mockTx.kategori.label}');
      print('  Nominal: Rp ${mockTx.nominal.toString().replaceAllMapped(RegExp(r"(\d{1,3})(?=(\d{3})+(?!\d))"), (m) => "${m[1]}.")}}');
      print('  Keterangan: ${mockTx.keterangan}');
      print('  Referensi: ${mockTx.referensi ?? "(tidak ada)"}');

      if (mounted) {
        ScaffoldMessenger.of(context).showSnackBar(
          const SnackBar(
            content: Text('Transaksi KAS berhasil disimpan.'),
            backgroundColor: AppColors.success,
          ),
        );
        context.pop(mockTx);
      }
    } catch (e) {
      if (mounted) {
        setState(() => _error = 'Gagal menyimpan transaksi: $e');
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

    return DoubleBackExitScope(
      child: PopScope(
        canPop: false,
        onPopInvokedWithResult: (didPop, result) async {
          if (didPop) return;
          await _confirmCancel();
        },
        child: Scaffold(
          appBar: AppBar(
            centerTitle: false,
            title: const Text('Tambah Kas Masuk'),
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
                      color: AppColors.danger.withAlpha(isDark ? 51 : 26),
                      borderRadius: BorderRadius.circular(AppRadius.base),
                      border: Border.all(
                        color: AppColors.danger.withAlpha(isDark ? 102 : 51),
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
                      color: AppColors.danger.withAlpha(isDark ? 38 : 20),
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
                                const Text('• ',
                                    style: TextStyle(fontSize: 16)),
                                Expanded(child: Text(e)),
                              ],
                            ),
                          ),
                        ),
                      ],
                    ),
                  ),

                const SectionHeader(title: 'Detail Transaksi'),
                const SizedBox(height: AppSpacing.sm),

                // Tanggal
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Tanggal',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    InkWell(
                      onTap: () async {
                        final picked = await showDatePicker(
                          context: context,
                          initialDate: _tanggalCtrl.text.isNotEmpty
                              ? (DateTime.tryParse(_tanggalCtrl.text.trim()) ??
                                  DateTime.now())
                              : DateTime.now(),
                          firstDate: DateTime(2020),
                          lastDate: DateTime.now().add(
                            const Duration(days: 365),
                          ),
                        );
                        if (picked != null) {
                          setState(() {
                            _tanggalCtrl.text = picked.toLocal()
                                .toString()
                                .split(' ')
                                .first;
                          });
                        }
                      },
                      child: Container(
                        padding: const EdgeInsets.all(AppSpacing.md),
                        decoration: BoxDecoration(
                          border: Border.all(color: AppColors.lightBorder),
                          borderRadius:
                              BorderRadius.circular(AppRadius.base),
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
                              _tanggalCtrl.text.isEmpty
                                  ? 'Pilih tanggal'
                                  : _tanggalCtrl.text,
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

                // Kategori dropdown
                Column(
                  crossAxisAlignment: CrossAxisAlignment.start,
                  children: [
                    Text(
                      'Kategori',
                      style: theme.textTheme.titleMedium?.copyWith(
                        fontWeight: FontWeight.w600,
                      ),
                    ),
                    const SizedBox(height: AppSpacing.sm),
                    Container(
                      decoration: BoxDecoration(
                        border: Border.all(color: AppColors.lightBorder),
                        borderRadius: BorderRadius.circular(AppRadius.base),
                      ),
                      child: DropdownButtonHideUnderline(
                        child: DropdownButton<String>(
                          value: _selectedKategori,
                          isExpanded: true,
                          hint: const Text('Pilih kategori'),
                          items: _kategoriOptions.map((kategori) {
                            return DropdownMenuItem(
                              value: kategori,
                              child: Text(kategori),
                            );
                          }).toList(),
                          onChanged: (value) {
                            setState(() => _selectedKategori = value);
                          },
                          icon: const Icon(Icons.arrow_drop_down, size: 20),
                        ),
                      ),
                    ),
                  ],
                ),
                const SizedBox(height: AppSpacing.lg),

                // Nominal
                AppTextField(
                  controller: _nominalCtrl,
                  label: 'Nominal (Rp)',
                  hintText: 'Masukkan nominal',
                  keyboardType: TextInputType.number,
                  inputFormatters: [
                    FilteringTextInputFormatter.digitsOnly,
                  ],
                  onChanged: (_) => setState(() {}),
                ),
                const SizedBox(height: AppSpacing.sm),
                // Live preview formatted amount
                if (_nominalCtrl.text.trim().isNotEmpty &&
                    int.tryParse(_nominalCtrl.text.trim()) != null)
                  Padding(
                    padding: const EdgeInsets.only(left: 16, bottom: AppSpacing.lg),
                    child: Text(
                      'Format: ${RupiahFormatter.format(int.parse(_nominalCtrl.text.trim()))}',
                      style: theme.textTheme.bodySmall?.copyWith(
                        color: AppColors.success,
                        fontWeight: FontWeight.w500,
                      ),
                    ),
                  ),

                // Keterangan
                AppTextField(
                  controller: _keteranganCtrl,
                  label: 'Keterangan',
                  hintText: 'Deskripsi transaksi',
                ),
                const SizedBox(height: AppSpacing.lg),

                // Referensi (optional)
                AppTextField(
                  controller: _referensiCtrl,
                  label: 'Referensi (opsional)',
                  hintText: 'No. bukti transfer, dll.',
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
                              color: AppColors.success.withValues(alpha: 0.15),
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
                              side: const BorderSide(color: AppColors.success),
                              foregroundColor: AppColors.success,
                              shape: RoundedRectangleBorder(
                                borderRadius:
                                    BorderRadius.circular(AppRadius.base),
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
                                        AppColors.success,
                                      ),
                                    ),
                                  )
                                : const Row(
                                    mainAxisAlignment: MainAxisAlignment.center,
                                    children: [
                                      Icon(Icons.check_circle_outline,
                                          size: 18),
                                      SizedBox(width: 6),
                                      Text('Simpan'),
                                    ],
                                  ),
                          ),
                        ),
                      ),
                    ),
                    const SizedBox(width: AppSpacing.sm),
                    Expanded(
                      child: SizedBox(
                        height: 48,
                        child: OutlinedButton(
                          onPressed: _saving ? null : _confirmCancel,
                          style: OutlinedButton.styleFrom(
                            side: BorderSide(
                                color: AppColors.lightTextMuted.withValues(
                                    alpha: isDark ? 0.3 : 0.2)),
                            foregroundColor: AppColors.lightTextMuted,
                            shape: RoundedRectangleBorder(
                              borderRadius: BorderRadius.circular(AppRadius.base),
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
                  ],
                ),
              ],
            ),
          ),
        ),
      ),
    );
  }
}
